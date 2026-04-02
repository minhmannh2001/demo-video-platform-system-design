package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"video-platform/demo/internal/models"
	mongostore "video-platform/demo/internal/store"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// --- fakes ---

type fakeS3 struct {
	objects map[string][]byte
	putKeys []string
	getErr  error
	putErr  error
}

func (f *fakeS3) GetObject(ctx context.Context, in *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	bucket := aws.ToString(in.Bucket)
	key := aws.ToString(in.Key)
	comp := bucket + "/" + key
	if data, ok := f.objects[comp]; ok {
		return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(data))}, nil
	}
	if data, ok := f.objects[key]; ok {
		return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(data))}, nil
	}
	return nil, fmt.Errorf("no such key")
}

func (f *fakeS3) PutObject(ctx context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	if f.putErr != nil {
		return nil, f.putErr
	}
	key := aws.ToString(in.Key)
	f.putKeys = append(f.putKeys, key)
	_, _ = io.ReadAll(in.Body)
	return &s3.PutObjectOutput{}, nil
}

type fakeStore struct {
	byID      map[string]*models.Video
	markReady []markReadyCall
	failedIDs []string
}

type markReadyCall struct {
	id, prefix, thumbnailKey string
	duration                 int
	renditions               []models.Rendition
}

func (f *fakeStore) GetByID(ctx context.Context, id string) (*models.Video, error) {
	if f.byID == nil {
		return nil, nil
	}
	return f.byID[id], nil
}

func (f *fakeStore) MarkReady(ctx context.Context, id, encodedPrefix string, durationSec int, thumbnailKey string, renditions []models.Rendition) error {
	f.markReady = append(f.markReady, markReadyCall{
		id:           id,
		prefix:       encodedPrefix,
		duration:     durationSec,
		thumbnailKey: thumbnailKey,
		renditions:   renditions,
	})
	return nil
}

func (f *fakeStore) MarkFailed(ctx context.Context, id string) error {
	f.failedIDs = append(f.failedIDs, id)
	return nil
}

type fakeEncoder struct {
	err error
}

func (e *fakeEncoder) EncodeToHLS(ctx context.Context, inputPath, outputDir string) error {
	if e.err != nil {
		return e.err
	}
	// Minimal valid-looking HLS tree (no real ffmpeg in unit test).
	if err := os.WriteFile(filepath.Join(outputDir, "master.m3u8"), []byte("#EXTM3U\n#EXT-X-VERSION:3\n#EXTINF:1.0,\nseg0.ts\n#EXT-X-ENDLIST\n"), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outputDir, "seg0.ts"), []byte{0, 0, 0}, 0o644)
}

type fakeCache struct {
	deleted []string
}

func (f *fakeCache) Del(ctx context.Context, id string) error {
	f.deleted = append(f.deleted, id)
	return nil
}

func testProcessor(t *testing.T, s3f *fakeS3, st *fakeStore, enc Encoder, rawBucket, encBucket string) *Processor {
	t.Helper()
	return NewProcessor(Deps{
		S3:            s3f,
		RawBucket:     rawBucket,
		EncodedBucket: encBucket,
		Store:         st,
		Encoder:       enc,
		Cache:         nil,
		TempDirParent: t.TempDir(),
	})
}

func TestHandleMessage_invalidJSON(t *testing.T) {
	p := testProcessor(t, &fakeS3{}, &fakeStore{}, &fakeEncoder{}, "raw", "enc")
	err := p.HandleMessage(context.Background(), `{`)
	if err == nil {
		t.Fatal("want error")
	}
}

func TestHandleMessage_missingVideoID(t *testing.T) {
	p := testProcessor(t, &fakeS3{}, &fakeStore{}, &fakeEncoder{}, "raw", "enc")
	err := p.HandleMessage(context.Background(), `{}`)
	if err == nil {
		t.Fatal("want error")
	}
}

func TestHandleMessage_videoNotInDB(t *testing.T) {
	id := primitive.NewObjectID().Hex()
	p := testProcessor(t, &fakeS3{}, &fakeStore{byID: map[string]*models.Video{}}, &fakeEncoder{}, "raw", "enc")
	err := p.HandleMessage(context.Background(), mustJSON(t, map[string]string{"video_id": id}))
	if err == nil {
		t.Fatal("want error")
	}
}

func TestHandleMessage_s3RawMissing(t *testing.T) {
	id := primitive.NewObjectID().Hex()
	st := &fakeStore{byID: map[string]*models.Video{
		id: {ID: id, RawS3Key: "videos/" + id + "/original.mp4", Status: models.StatusProcessing},
	}}
	s3f := &fakeS3{objects: map[string][]byte{}}
	p := testProcessor(t, s3f, st, &fakeEncoder{}, "raw", "enc")
	err := p.HandleMessage(context.Background(), mustJSON(t, map[string]string{"video_id": id}))
	if err == nil {
		t.Fatal("want error")
	}
	if len(st.failedIDs) != 1 || st.failedIDs[0] != id {
		t.Fatalf("MarkFailed: %#v", st.failedIDs)
	}
}

func TestHandleMessage_encodeError(t *testing.T) {
	id := primitive.NewObjectID().Hex()
	rawKey := "videos/" + id + "/original.mp4"
	st := &fakeStore{byID: map[string]*models.Video{
		id: {ID: id, RawS3Key: rawKey, Status: models.StatusProcessing},
	}}
	s3f := &fakeS3{objects: map[string][]byte{"raw/" + rawKey: []byte("binary")}}
	p := testProcessor(t, s3f, st, &fakeEncoder{err: fmt.Errorf("encode boom")}, "raw", "enc")
	err := p.HandleMessage(context.Background(), mustJSON(t, map[string]string{"video_id": id}))
	if err == nil {
		t.Fatal("want error")
	}
	if len(st.failedIDs) != 1 {
		t.Fatalf("want MarkFailed, got %#v", st.failedIDs)
	}
}

func TestHandleMessage_success(t *testing.T) {
	id := primitive.NewObjectID().Hex()
	rawKey := "videos/" + id + "/original.mp4"
	st := &fakeStore{byID: map[string]*models.Video{
		id: {ID: id, RawS3Key: rawKey, Status: models.StatusProcessing},
	}}
	s3f := &fakeS3{objects: map[string][]byte{"raw/" + rawKey: []byte("fakevideo")}}
	ca := &fakeCache{}
	p := NewProcessor(Deps{
		S3:            s3f,
		RawBucket:     "raw",
		EncodedBucket: "enc",
		Store:         st,
		Encoder:       &fakeEncoder{},
		Cache:         ca,
		TempDirParent: t.TempDir(),
	})
	err := p.HandleMessage(context.Background(), mustJSON(t, map[string]string{"video_id": id}))
	if err != nil {
		t.Fatal(err)
	}
	if len(st.markReady) != 1 {
		t.Fatalf("MarkReady calls: %+v", st.markReady)
	}
	mr := st.markReady[0]
	if mr.id != id || mr.duration < 0 {
		t.Fatalf("bad MarkReady: %+v", mr)
	}
	if !strings.HasPrefix(mr.prefix, "videos/"+id+"/hls") {
		t.Fatalf("prefix %q", mr.prefix)
	}
	var hasMaster, hasSeg bool
	for _, k := range s3f.putKeys {
		if strings.HasSuffix(k, "/hls/master.m3u8") {
			hasMaster = true
		}
		if strings.HasSuffix(k, "/hls/seg0.ts") {
			hasSeg = true
		}
	}
	if !hasMaster || !hasSeg {
		t.Fatalf("put keys: %v", s3f.putKeys)
	}
	if len(ca.deleted) != 1 || ca.deleted[0] != id {
		t.Fatalf("cache del: %#v", ca.deleted)
	}
}

func TestParseJobBody(t *testing.T) {
	id := primitive.NewObjectID().Hex()
	j, err := parseJobBody(mustJSON(t, map[string]string{"video_id": id}))
	if err != nil || j.VideoID != id {
		t.Fatalf("%+v %v", j, err)
	}
	_, err = parseJobBody(`{}`)
	if err == nil {
		t.Fatal("want err")
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Compile-time: Mongo VideoStore implements worker.VideoStore.
var _ VideoStore = (*mongostore.VideoStore)(nil)
