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
	"time"

	"video-platform/internal/cache"
	"video-platform/internal/config"
	"video-platform/internal/models"
	"video-platform/internal/search/esclient"
	"video-platform/internal/store"
	"video-platform/internal/videometaqueue"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson/primitive"
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

func (f *fakeS3) DeleteObject(ctx context.Context, in *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	if f.objects != nil {
		delete(f.objects, aws.ToString(in.Key))
	}
	return &s3.DeleteObjectOutput{}, nil
}

func (f *fakeS3) ListObjectsV2(ctx context.Context, in *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	prefix := aws.ToString(in.Prefix)
	var keys []types.Object
	if f.objects != nil {
		for k := range f.objects {
			if strings.HasPrefix(k, prefix) {
				kCopy := k
				keys = append(keys, types.Object{Key: aws.String(kCopy)})
			}
		}
	}
	return &s3.ListObjectsV2Output{Contents: keys}, nil
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

type recordingMetaPub struct {
	events []videometaqueue.Event
}

func (r *recordingMetaPub) Publish(ctx context.Context, ev videometaqueue.Event) error {
	r.events = append(r.events, ev)
	return nil
}

type fakeVideoSearch struct {
	result *esclient.SearchPublishedResult
	err    error
}

func (f *fakeVideoSearch) SearchPublishedVideos(ctx context.Context, q string, from, size int, withHighlight bool) (*esclient.SearchPublishedResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

type fakeStore struct {
	byID    map[string]*models.Video
	createE error
	listE   error
	getE    error
	updateE error
}

func (f *fakeStore) Create(ctx context.Context, v *models.Video) error {
	if f.createE != nil {
		return f.createE
	}
	if f.byID == nil {
		f.byID = make(map[string]*models.Video)
	}
	if v.CreatedAt.IsZero() {
		now := time.Now().UTC()
		v.CreatedAt = now
		v.UpdatedAt = now
	}
	f.byID[v.ID] = v
	return nil
}

func (f *fakeStore) GetByID(ctx context.Context, id string) (*models.Video, error) {
	if f.getE != nil {
		return nil, f.getE
	}
	if _, err := primitive.ObjectIDFromHex(id); err != nil {
		return nil, err
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

func (f *fakeStore) UpdateMetadata(ctx context.Context, id, title, description, visibility string) error {
	if f.updateE != nil {
		return f.updateE
	}
	if _, err := primitive.ObjectIDFromHex(id); err != nil {
		return err
	}
	if f.byID == nil {
		return store.ErrVideoNotFound
	}
	v, ok := f.byID[id]
	if !ok {
		return store.ErrVideoNotFound
	}
	v.Title = title
	v.Description = description
	if visibility != "" {
		v.Visibility = visibility
	}
	v.UpdatedAt = time.Now().UTC()
	return nil
}

func (f *fakeStore) DeleteByID(ctx context.Context, id string) (bool, error) {
	if _, err := primitive.ObjectIDFromHex(id); err != nil {
		return false, err
	}
	if f.byID == nil {
		return false, nil
	}
	if _, ok := f.byID[id]; !ok {
		return false, nil
	}
	delete(f.byID, id)
	return true, nil
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

func (h *hitCache) Del(ctx context.Context, id string) error { return nil }

func TestUpload_validation(t *testing.T) {
	h := New(testCfg(), &fakeS3{}, &fakeSQS{}, "http://q", "", &fakeStore{}, nil, nil, nil, nil)
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
	meta := &recordingMetaPub{}
	h := New(testCfg(), s3f, sqsf, "http://queue", "http://meta-q", st, nil, meta, nil, nil)
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
	if len(meta.events) != 1 || meta.events[0].VideoID != out.ID || meta.events[0].Op != videometaqueue.OpCreated {
		t.Fatalf("metadata events: %+v", meta.events)
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
	h := New(testCfg(), &fakeS3{}, &fakeSQS{}, "q", "", &fakeStore{byID: map[string]*models.Video{}}, nil, nil, nil, nil)
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

func TestPatchVideo_success(t *testing.T) {
	id := "507f1f77bcf86cd799439011"
	st := &fakeStore{byID: map[string]*models.Video{
		id: {ID: id, Title: "old", Description: "d", Status: models.StatusReady},
	}}
	meta := &recordingMetaPub{}
	h := New(testCfg(), &fakeS3{}, &fakeSQS{}, "q", "meta", st, nil, meta, nil, nil)
	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)

	body := `{"title":"new title","description":"nd"}`
	req, _ := http.NewRequest(http.MethodPatch, srv.URL+"/videos/"+id, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, b)
	}
	var got models.Video
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Title != "new title" || got.Description != "nd" {
		t.Fatalf("%+v", got)
	}
	if len(meta.events) != 1 || meta.events[0].Op != videometaqueue.OpUpdated {
		t.Fatalf("metadata: %+v", meta.events)
	}
}

func TestGetVideo_cacheHit(t *testing.T) {
	id := "507f1f77bcf86cd799439011"
	v := &models.Video{ID: id, Title: "cached", Status: models.StatusReady}
	h := New(testCfg(), &fakeS3{}, &fakeSQS{}, "q", "", &fakeStore{}, &hitCache{v: v}, nil, nil, nil)
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
	wantThumb := "http://localhost:8080/stream/" + id + "/thumbnail.jpg"
	if got.ThumbnailURL != wantThumb {
		t.Fatalf("thumbnail_url: %q want %q", got.ThumbnailURL, wantThumb)
	}
}

func TestGetVideo_notFound(t *testing.T) {
	h := New(testCfg(), &fakeS3{}, &fakeSQS{}, "q", "", &fakeStore{byID: map[string]*models.Video{}}, nil, nil, nil, nil)
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
		id: {
			ID:           id,
			Status:       models.StatusReady,
			ThumbnailKey: "thumbnail.jpg",
			Renditions: []models.Rendition{
				{Quality: "360p", Width: 640, Height: 360, Key: "360p/prog.m3u8"},
				{Quality: "720p", Width: 1280, Height: 720, Key: "720p/prog.m3u8"},
			},
		},
	}}
	h := New(testCfg(), &fakeS3{}, &fakeSQS{}, "q", "", st, nil, nil, nil, nil)
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
	if w.ThumbnailURL != "http://localhost:8080/stream/"+id+"/thumbnail.jpg" {
		t.Fatalf("thumbnail_url: %q", w.ThumbnailURL)
	}
	if len(w.Qualities) != 3 || w.Qualities[0] != "360p" || w.Qualities[1] != "720p" || w.Qualities[2] != "auto" {
		t.Fatalf("qualities: %#v", w.Qualities)
	}
	if len(w.Renditions) != 2 {
		t.Fatalf("renditions: %+v", w.Renditions)
	}
	if w.Renditions[0].PlaylistURL != "http://localhost:8080/stream/"+id+"/360p/prog.m3u8" {
		t.Fatalf("360p url: %q", w.Renditions[0].PlaylistURL)
	}
}

func TestDeleteVideo_success(t *testing.T) {
	id := "507f1f77bcf86cd799439011"
	rawKey := "videos/" + id + "/original.mp4"
	encKey := "videos/" + id + "/hls/master.m3u8"
	s3f := &fakeS3{objects: map[string][]byte{
		rawKey: []byte("raw"),
		encKey: []byte("#EXTM3U"),
	}}
	st := &fakeStore{byID: map[string]*models.Video{
		id: {
			ID:            id,
			RawS3Key:      rawKey,
			EncodedPrefix: "videos/" + id + "/hls/",
			Status:        models.StatusReady,
		},
	}}
	meta := &recordingMetaPub{}
	h := New(testCfg(), s3f, &fakeSQS{}, "q", "http://meta-q", st, nil, meta, nil, nil)
	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/videos/"+id, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, body)
	}
	if _, ok := st.byID[id]; ok {
		t.Fatal("video should be removed from store")
	}
	if len(s3f.objects) != 0 {
		t.Fatalf("s3 should be empty, got %v", s3f.objects)
	}
	if len(meta.events) != 1 || meta.events[0].Op != videometaqueue.OpDeleted || meta.events[0].VideoID != id {
		t.Fatalf("metadata events: %+v", meta.events)
	}
}

func TestDeleteVideo_notFound(t *testing.T) {
	id := "507f1f77bcf86cd799439011"
	h := New(testCfg(), &fakeS3{}, &fakeSQS{}, "q", "", &fakeStore{byID: map[string]*models.Video{}}, nil, nil, nil, nil)
	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/videos/"+id, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
}

func TestDeleteVideo_invalidID(t *testing.T) {
	h := New(testCfg(), &fakeS3{}, &fakeSQS{}, "q", "", &fakeStore{}, nil, nil, nil, nil)
	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/videos/not-a-valid-objectid", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

func TestStreamObject_success(t *testing.T) {
	id := "507f1f77bcf86cd799439011"
	key := "videos/" + id + "/hls/master.m3u8"
	s3f := &fakeS3{objects: map[string][]byte{key: []byte("#EXTM3U\n")}}
	h := New(testCfg(), s3f, &fakeSQS{}, "q", "", &fakeStore{}, nil, nil, nil, nil)
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
	h := New(testCfg(), &fakeS3{}, &fakeSQS{}, "q", "", &fakeStore{}, nil, nil, nil, nil)
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

	t.Run("OPTIONS preflight allows traceparent", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/videos", nil)
		req.Header.Set("Origin", "http://localhost:5173")
		req.Header.Set("Access-Control-Request-Method", "GET")
		req.Header.Set("Access-Control-Request-Headers", "traceparent")
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("code %d", rec.Code)
		}
		allow := rec.Header().Get("Access-Control-Allow-Headers")
		if !strings.Contains(strings.ToLower(allow), "traceparent") {
			t.Fatalf("Allow-Headers missing traceparent: %q", allow)
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
	h := New(testCfg(), &fakeS3{}, &fakeSQS{}, "q", "", &fakeStore{}, nil, nil, nil, nil)
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

func TestSearchVideos_unavailable(t *testing.T) {
	h := New(testCfg(), &fakeS3{}, &fakeSQS{}, "q", "", &fakeStore{}, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/videos/search?q=foo", nil)
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d, want 503", rec.Code)
	}
}

func TestSearchVideos_missingQuery(t *testing.T) {
	h := New(testCfg(), &fakeS3{}, &fakeSQS{}, "q", "", &fakeStore{}, nil, nil, &fakeVideoSearch{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/videos/search", nil)
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", rec.Code)
	}
}

func TestSearchVideos_ok(t *testing.T) {
	score := 12.5
	want := &esclient.SearchPublishedResult{
		Total: 1,
		From:  0,
		Size:  20,
		Hits: []esclient.SearchPublishedHit{
			{VideoID: "vid-1", Score: &score},
		},
	}
	h := New(testCfg(), &fakeS3{}, &fakeSQS{}, "q", "", &fakeStore{}, nil, nil, &fakeVideoSearch{result: want}, nil)
	req := httptest.NewRequest(http.MethodGet, "/videos/search?q=hello&from=0&size=20", nil)
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rec.Code)
	}
	var got esclient.SearchPublishedResult
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Total != want.Total || len(got.Hits) != 1 || got.Hits[0].VideoID != "vid-1" {
		t.Fatalf("body mismatch: %+v", got)
	}
	if got.Hits[0].Score == nil || *got.Hits[0].Score != score {
		t.Fatalf("score = %v, want %v", got.Hits[0].Score, score)
	}
}

// Compile-time: production types satisfy handler interfaces.
var (
	_ VideoRepository = (*store.VideoStore)(nil)
	_ VideoCacher     = (*cache.VideoCache)(nil)
	_ VideoSearch     = (*esclient.Client)(nil)
	_ VideoSearch     = (*cache.CachedPublishedSearch)(nil)
)
