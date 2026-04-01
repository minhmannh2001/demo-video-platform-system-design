package videosearchsync

import (
	"context"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// VersionTracker drops stale metadata events: only applies when correlation_version > last committed (or no prior commit).
type VersionTracker interface {
	ShouldApply(ctx context.Context, videoID string, correlationVersion int64) (bool, error)
	Commit(ctx context.Context, videoID string, correlationVersion int64) error
	Clear(ctx context.Context, videoID string) error
}

// NoopVersionTracker always applies (no cross-replica coordination).
type NoopVersionTracker struct{}

func (NoopVersionTracker) ShouldApply(context.Context, string, int64) (bool, error) { return true, nil }
func (NoopVersionTracker) Commit(context.Context, string, int64) error               { return nil }
func (NoopVersionTracker) Clear(context.Context, string) error                       { return nil }

// RedisVersionTracker stores last applied correlation_version per video_id.
type RedisVersionTracker struct {
	rdb *redis.Client
}

func NewRedisVersionTracker(rdb *redis.Client) *RedisVersionTracker {
	return &RedisVersionTracker{rdb: rdb}
}

func (r *RedisVersionTracker) key(videoID string) string {
	return "videosearch:cver:" + videoID
}

// ShouldApply returns true if this event is newer than the last applied version (or first time).
// Duplicate deliveries with the same correlation_version return false after a successful Commit.
func (r *RedisVersionTracker) ShouldApply(ctx context.Context, videoID string, correlationVersion int64) (bool, error) {
	if r == nil || r.rdb == nil {
		return true, nil
	}
	cur, err := r.rdb.Get(ctx, r.key(videoID)).Int64()
	if errors.Is(err, redis.Nil) {
		return true, nil
	}
	if err != nil {
		return true, fmt.Errorf("videosearchsync redis get: %w", err)
	}
	return correlationVersion > cur, nil
}

func (r *RedisVersionTracker) Commit(ctx context.Context, videoID string, correlationVersion int64) error {
	if r == nil || r.rdb == nil {
		return nil
	}
	key := r.key(videoID)
	cur, err := r.rdb.Get(ctx, key).Int64()
	if errors.Is(err, redis.Nil) || correlationVersion >= cur {
		return r.rdb.Set(ctx, key, correlationVersion, 0).Err()
	}
	return nil
}

func (r *RedisVersionTracker) Clear(ctx context.Context, videoID string) error {
	if r == nil || r.rdb == nil {
		return nil
	}
	return r.rdb.Del(ctx, r.key(videoID)).Err()
}
