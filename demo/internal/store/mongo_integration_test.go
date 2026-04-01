//go:build integration

package store

import (
	"context"
	"testing"

	"video-platform/demo/internal/models"

	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// TestVideoStore_Mongo_Integration exercises Create, GetByID, List, MarkReady, MarkFailed against a real MongoDB (Docker).
// Run: go test -tags=integration ./internal/store/ -v
// Requires: Docker daemon available to Testcontainers.
func TestVideoStore_Mongo_Integration(t *testing.T) {
	ctx := context.Background()

	mongoC, err := mongodb.Run(ctx, "mongo:7")
	if err != nil {
		t.Fatalf("start mongo: %v", err)
	}
	t.Cleanup(func() {
		_ = mongoC.Terminate(context.Background())
	})

	uri, err := mongoC.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}

	client, err := Connect(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		t.Fatal(err)
	}

	db := client.Database("video_demo_integration")
	_ = db.Drop(ctx)
	t.Cleanup(func() { _ = db.Drop(context.Background()) })

	s := NewVideoStore(db)

	id := primitive.NewObjectID().Hex()
	v := &models.Video{
		ID:          id,
		Title:       "integration",
		Description: "d",
		Uploader:    "u",
		RawS3Key:    "videos/" + id + "/original.mp4",
		Status:      models.StatusProcessing,
	}
	if err := s.Create(ctx, v); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Title != "integration" || got.Status != models.StatusProcessing {
		t.Fatalf("GetByID: %+v", got)
	}
	if got.Visibility != models.VisibilityPublic {
		t.Fatalf("default visibility: %+v", got)
	}

	if err := s.UpdateMetadata(ctx, id, "integration", "updated desc", models.VisibilityPrivate); err != nil {
		t.Fatal(err)
	}
	got, err = s.GetByID(ctx, id)
	if err != nil || got == nil || got.Description != "updated desc" || got.Visibility != models.VisibilityPrivate {
		t.Fatalf("after UpdateMetadata: %+v err=%v", got, err)
	}

	if err := s.MarkReady(ctx, id, "videos/"+id+"/hls/", 42); err != nil {
		t.Fatal(err)
	}
	got, err = s.GetByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != models.StatusReady || got.EncodedPrefix == "" || got.DurationSec != 42 {
		t.Fatalf("after MarkReady: %+v", got)
	}

	if err := s.MarkFailed(ctx, id); err != nil {
		t.Fatal(err)
	}
	got, err = s.GetByID(ctx, id)
	if err != nil || got.Status != models.StatusFailed {
		t.Fatalf("after MarkFailed: %+v err=%v", got, err)
	}

	// List: insert second doc, expect desc _id
	id2 := primitive.NewObjectID().Hex()
	v2 := &models.Video{
		ID:          id2,
		Title:       "second",
		Description: "",
		Uploader:    "",
		RawS3Key:    "videos/" + id2 + "/original.mp4",
		Status:      models.StatusProcessing,
	}
	if err := s.Create(ctx, v2); err != nil {
		t.Fatal(err)
	}
	list, err := s.List(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("list len %d", len(list))
	}
	// Newest _id first
	if list[0].ID != id2 || list[1].ID != id {
		t.Fatalf("order: %#v", list)
	}
}

func TestVideoStore_Mongo_GetByID_invalidHex(t *testing.T) {
	ctx := context.Background()
	mongoC, err := mongodb.Run(ctx, "mongo:7")
	if err != nil {
		t.Fatalf("start mongo: %v", err)
	}
	t.Cleanup(func() { _ = mongoC.Terminate(context.Background()) })

	uri, err := mongoC.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}
	client, err := Connect(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()

	s := NewVideoStore(client.Database("video_demo_integration_invalid"))
	_, err = s.GetByID(ctx, "not-a-valid-objectid")
	if err == nil {
		t.Fatal("want error for invalid id")
	}
}
