package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"video-platform/demo/internal/cache"
	"video-platform/demo/internal/config"
	"video-platform/demo/internal/models"
	"video-platform/demo/internal/store"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/redis/go-redis/v9"
)

func testCfg() config.Config {
	return config.Config{
		S3RawBucket:     "raw-bucket",
		S3EncodedBucket: "enc-bucket",
		PublicBaseURL:   "http://localhost:8080",
	}
}

type fakeS3 struct {
	objects map[string][]byte
	putErr  error
	getErr  error
}

func (f *fakeS3) PutObject(ctx context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	if f.putErr != nil {
		return nil, f.putErr
	}
	if f.objects == nil {
		f.objects = make(map[string][]byte)
	}
	key := aws.ToString(in.Key)
	data, _ := io.ReadAll(in.Body)
	f.objects[key] = data
	return &s3.PutObjectOutput{}, nil
}

func (f *fakeS3) GetObject(ctx context.Context, in *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	key := aws.ToString(in.Key)
	data := f.objects[key]
	if data == nil {
		return nil, fmt.Errorf("not found")
	}
	return &s3.GetObjectOutput{
		Body:        io.NopCloser(bytes.NewReader(data)),
		ContentType: aws.String(""),
	}, nil
}

func (f *fakeS3) ListBuckets(ctx context.Context, in *s3.ListBucketsInput, _ ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
	return &s3.ListBucketsOutput{}, nil
}

type fakeSQS struct {
	bodies [][]byte
	err    error
}

func (f *fakeSQS) SendMessage(ctx context.Context, in *sqs.SendMessageInput, _ ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.bodies = append(f.bodies, []byte(aws.ToString(in.MessageBody)))
	return &sqs.SendMessageOutput{}, nil
}

type fakeStore struct {
	byID    map[string]*models.Video
	createE error
	listE   error
	getE    error
}

func (f *fakeStore) Create(ctx context.Context, v *models.Video) error {
	if f.createE != nil {
		return f.createE
	}
	if f.byID == nil {
		f.byID = make(map[string]*models.Video)
	}
	f.byID[v.ID] = v
	return nil
}

func (f *fakeStore) GetByID(ctx context.Context, id string) (*models.Video, error) {
	if f.getE != nil {
		return nil, f.getE
	}
	if f.byID == nil {
		return nil, nil
	}
	return f.byID[id], nil
}

func (f *fakeStore) List(ctx context.Context, limit int64) ([]models.Video, error) {
	if f.listE != nil {
		return nil, f.listE
	}
	out := make([]models.Video, 0, len(f.byID))
	for _, v := range f.byID {
		out = append(out, *v)
	}
	return out, nil
}

type hitCache struct {
	v *models.Video
}

func (h *hitCache) Get(ctx context.Context, id string) (*models.Video, error) {
	if h.v != nil && h.v.ID == id {
		return h.v, nil
	}
	return nil, redis.Nil
}

func (h *hitCache) Set(ctx context.Context, v *models.Video) error { return nil }

func TestUpload_validation(t *testing.T) {
	h := New(testCfg(), &fakeS3{}, &fakeSQS{}, "http://q", &fakeStore{}, nil)
	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)

	resp, err := http.Post(srv.URL+"/videos/upload", "multipart/form-data", strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

func TestUpload_success(t *testing.T) {
	s3f := &fakeS3{}
	sqsf := &fakeSQS{}
	st := &fakeStore{}
	h := New(testCfg(), s3f, sqsf, "http://queue", st, nil)
	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)

	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	_ = mw.WriteField("title", "Hello")
	_ = mw.WriteField("description", "d")
	_ = mw.WriteField("uploader", "u")
	fw, err := mw.CreateFormFile("file", "clip.mp4")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write([]byte("fakevideo"))
	_ = mw.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/videos/upload", &b)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, body)
	}
	var out struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.ID == "" || out.Status != models.StatusProcessing {
		t.Fatalf("bad body: %+v", out)
	}
	if len(sqsf.bodies) != 1 {
		t.Fatalf("sqs messages: %d", len(sqsf.bodies))
	}
	var env map[string]string
	if err := json.Unmarshal(sqsf.bodies[0], &env); err != nil || env["video_id"] != out.ID {
		t.Fatalf("sqs body: %s", sqsf.bodies[0])
	}
	var found bool
	for k := range s3f.objects {
		if strings.Contains(k, out.ID) && strings.HasPrefix(k, "videos/") {
			found = true
		}
	}
	if !found {
		t.Fatalf("s3 keys: %v", s3f.objects)
	}
}

func TestListVideos_empty(t *testing.T) {
	h := New(testCfg(), &fakeS3{}, &fakeSQS{}, "q", &fakeStore{byID: map[string]*models.Video{}}, nil)
	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)
	resp, err := http.Get(srv.URL + "/videos")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var list []models.Video
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("want empty list, got %d", len(list))
	}
}

func TestGetVideo_cacheHit(t *testing.T) {
	id := "507f1f77bcf86cd799439011"
	v := &models.Video{ID: id, Title: "cached", Status: models.StatusReady}
	h := New(testCfg(), &fakeS3{}, &fakeSQS{}, "q", &fakeStore{}, &hitCache{v: v})
	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)
	resp, err := http.Get(srv.URL + "/videos/" + id)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var got models.Video
	json.NewDecoder(resp.Body).Decode(&got)
	if got.Title != "cached" {
		t.Fatalf("got %+v", got)
	}
}

func TestGetVideo_notFound(t *testing.T) {
	h := New(testCfg(), &fakeS3{}, &fakeSQS{}, "q", &fakeStore{byID: map[string]*models.Video{}}, nil)
	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)
	resp, err := http.Get(srv.URL + "/videos/507f1f77bcf86cd799439011")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
}

func TestWatch_ready(t *testing.T) {
	id := "507f1f77bcf86cd799439011"
	st := &fakeStore{byID: map[string]*models.Video{
		id: {ID: id, Status: models.StatusReady},
	}}
	h := New(testCfg(), &fakeS3{}, &fakeSQS{}, "q", st, nil)
	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)
	resp, err := http.Get(srv.URL + "/videos/" + id + "/watch")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var w models.WatchResponse
	json.NewDecoder(resp.Body).Decode(&w)
	if w.ManifestURL != "http://localhost:8080/stream/"+id+"/master.m3u8" {
		t.Fatalf("manifest: %q", w.ManifestURL)
	}
}

func TestStreamObject_success(t *testing.T) {
	id := "507f1f77bcf86cd799439011"
	key := "videos/" + id + "/hls/master.m3u8"
	s3f := &fakeS3{objects: map[string][]byte{key: []byte("#EXTM3U\n")}}
	h := New(testCfg(), s3f, &fakeSQS{}, "q", &fakeStore{}, nil)
	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)
	resp, err := http.Get(srv.URL + "/stream/" + id + "/master.m3u8")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/vnd.apple.mpegurl" {
		t.Fatalf("content-type: %q", ct)
	}
}

func TestStreamObject_invalidPath(t *testing.T) {
	id := "507f1f77bcf86cd799439011"
	h := New(testCfg(), &fakeS3{}, &fakeSQS{}, "q", &fakeStore{}, nil)
	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)
	resp, err := http.Get(srv.URL + "/stream/" + id + "/../x")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

func TestCORSMiddleware(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrapped := CORSMiddleware([]string{"http://localhost:5173"})(h)

	t.Run("OPTIONS", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/", nil)
		req.Header.Set("Origin", "http://localhost:5173")
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("code %d", rec.Code)
		}
	})

	t.Run("GET sets ACAO", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Origin", "http://localhost:5173")
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
		if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
			t.Fatalf("missing ACAO")
		}
	})

	t.Run("unknown origin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Origin", "http://evil.com")
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
		if rec.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatal("should not set ACAO for unknown origin")
		}
	})
}

func TestHealth(t *testing.T) {
	h := New(testCfg(), &fakeS3{}, &fakeSQS{}, "q", &fakeStore{}, nil)
	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)
	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatal(resp.StatusCode)
	}
}

// Compile-time: production types satisfy handler interfaces.
var (
	_ VideoRepository = (*store.VideoStore)(nil)
	_ VideoCacher     = (*cache.VideoCache)(nil)
)
