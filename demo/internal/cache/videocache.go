package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"video-platform/demo/internal/models"

	"github.com/redis/go-redis/v9"
)

type VideoCache struct {
	rdb *redis.Client
	ttl time.Duration
}

func New(addr string, ttl time.Duration) *VideoCache {
	return &VideoCache{
		rdb: redis.NewClient(&redis.Options{Addr: addr}),
		ttl: ttl,
	}
}

func (c *VideoCache) key(id string) string {
	return fmt.Sprintf("video:%s", id)
}

func (c *VideoCache) Get(ctx context.Context, id string) (*models.Video, error) {
	if c == nil || c.rdb == nil {
		return nil, redis.Nil
	}
	s, err := c.rdb.Get(ctx, c.key(id)).Result()
	if err != nil {
		return nil, err
	}
	var v models.Video
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil, err
	}
	return &v, nil
}

func (c *VideoCache) Set(ctx context.Context, v *models.Video) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, c.key(v.ID), b, c.ttl).Err()
}

func (c *VideoCache) Del(ctx context.Context, id string) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Del(ctx, c.key(id)).Err()
}

func (c *VideoCache) Ping(ctx context.Context) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return c.rdb.Ping(ctx).Err()
}

// Redis exposes the underlying client for composable caches (e.g. search result cache).
func (c *VideoCache) Redis() *redis.Client {
	if c == nil {
		return nil
	}
	return c.rdb
}
