package cache

import (
	"context"
	"testing"
	"time"

	"video-platform/internal/models"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestVideoCache_roundTrip(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)

	c := New(mr.Addr(), time.Minute)
	ctx := context.Background()
	v := &models.Video{
		ID:     "507f1f77bcf86cd799439011",
		Title:  "t",
		Status: models.StatusProcessing,
	}
	if err := c.Set(ctx, v); err != nil {
		t.Fatal(err)
	}
	got, err := c.Get(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Title != "t" || got.ID != v.ID {
		t.Fatalf("got %+v", got)
	}
}

func TestVideoCache_miss(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)

	c := New(mr.Addr(), time.Minute)
	_, err = c.Get(context.Background(), "missing")
	if err != redis.Nil {
		t.Fatalf("want redis.Nil, got %v", err)
	}
}
