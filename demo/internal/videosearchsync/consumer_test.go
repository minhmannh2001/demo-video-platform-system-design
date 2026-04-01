package videosearchsync

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"video-platform/demo/internal/models"
	"video-platform/demo/internal/search"
	"video-platform/demo/internal/videometaqueue"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type fakeReader struct {
	v *models.Video
}

func (f *fakeReader) GetByID(ctx context.Context, id string) (*models.Video, error) {
	if f.v == nil || f.v.ID != id {
		return nil, nil
	}
	return f.v, nil
}

type fakeIndex struct {
	upserts []search.VideoSearchDoc
	deletes []string
	err     error
}

func (f *fakeIndex) UpsertVideo(ctx context.Context, doc *search.VideoSearchDoc) error {
	if f.err != nil {
		return f.err
	}
	f.upserts = append(f.upserts, *doc)
	return nil
}

func (f *fakeIndex) DeleteVideo(ctx context.Context, videoID string) error {
	if f.err != nil {
		return f.err
	}
	f.deletes = append(f.deletes, videoID)
	return nil
}

func TestConsumer_Handle_upsert(t *testing.T) {
	id := primitive.NewObjectID().Hex()
	now := time.Date(2026, 5, 1, 12, 0, 0, 100, time.UTC)
	v := &models.Video{
		ID: id, Title: "t", Description: "d", Uploader: "u",
		Status: models.StatusReady, Visibility: models.VisibilityPublic,
		CreatedAt: now, UpdatedAt: now,
	}
	idx := &fakeIndex{}
	c := &Consumer{
		Store:    &fakeReader{v: v},
		Index:    idx,
		Versions: NoopVersionTracker{},
	}
	ev := videometaqueue.NewEvent(id, videometaqueue.OpUpdated, now)
	b, _ := json.Marshal(ev)
	if err := c.Handle(context.Background(), string(b)); err != nil {
		t.Fatal(err)
	}
	if len(idx.upserts) != 1 || idx.upserts[0].VideoID != id {
		t.Fatalf("%+v", idx.upserts)
	}
}

func TestConsumer_Handle_delete(t *testing.T) {
	id := primitive.NewObjectID().Hex()
	idx := &fakeIndex{}
	c := &Consumer{Store: &fakeReader{}, Index: idx, Versions: NoopVersionTracker{}}
	ev := videometaqueue.NewEvent(id, videometaqueue.OpDeleted, time.Now().UTC())
	b, _ := json.Marshal(ev)
	if err := c.Handle(context.Background(), string(b)); err != nil {
		t.Fatal(err)
	}
	if len(idx.deletes) != 1 || idx.deletes[0] != id {
		t.Fatalf("%v", idx.deletes)
	}
}
