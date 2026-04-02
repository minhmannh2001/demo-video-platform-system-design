//go:build integration

package cache

import (
	"context"
	"net/url"
	"testing"
	"time"

	"video-platform/internal/models"

	"github.com/redis/go-redis/v9"
	redistc "github.com/testcontainers/testcontainers-go/modules/redis"
)

// TestVideoCache_Redis_Integration uses a real Redis from Testcontainers.
// Run: go test -tags=integration ./internal/cache/ -v
// Requires: Docker.
func TestVideoCache_Redis_Integration(t *testing.T) {
	ctx := context.Background()

	redisC, err := redistc.Run(ctx, "redis:7")
	if err != nil {
		t.Fatalf("start redis: %v", err)
	}
	t.Cleanup(func() {
		_ = redisC.Terminate(context.Background())
	})

	connStr, err := redisC.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}
	addr := redisAddrFromURL(connStr)

	c := New(addr, 2*time.Minute)
	if err := c.Ping(ctx); err != nil {
		t.Fatal(err)
	}

	v := &models.Video{
		ID:     "507f1f77bcf86cd799439011",
		Title:  "from integration",
		Status: models.StatusReady,
	}
	if err := c.Set(ctx, v); err != nil {
		t.Fatal(err)
	}
	got, err := c.Get(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != v.Title {
		t.Fatalf("got %+v", got)
	}

	if err := c.Del(ctx, v.ID); err != nil {
		t.Fatal(err)
	}
	_, err = c.Get(ctx, v.ID)
	if err != redis.Nil {
		t.Fatalf("want miss, got %v", err)
	}
}

func redisAddrFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err == nil && u.Host != "" {
		return u.Host
	}
	return raw
}
