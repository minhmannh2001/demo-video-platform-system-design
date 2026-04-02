package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"video-platform/internal/search/esclient"
	"video-platform/internal/tracing"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
)

const publishedSearchFilterVersion = "published:v1"

// PublishedSearchBackend runs catalog search against Elasticsearch.
type PublishedSearchBackend interface {
	SearchPublishedVideos(ctx context.Context, q string, from, size int, withHighlight bool) (*esclient.SearchPublishedResult, error)
}

// CachedPublishedSearch reduces ES QPS for repeated GET /videos/search queries.
type CachedPublishedSearch struct {
	inner   PublishedSearchBackend
	rdb     *redis.Client
	ttl     time.Duration
	indexID string
}

// NewCachedPublishedSearch wraps inner with Redis. ttl must be > 0; indexName identifies the ES index in the cache key.
// If rdb is nil, calls are passed through to inner.
func NewCachedPublishedSearch(inner PublishedSearchBackend, rdb *redis.Client, ttl time.Duration, indexName string) *CachedPublishedSearch {
	idx := strings.TrimSpace(indexName)
	if idx == "" {
		idx = "videos"
	}
	return &CachedPublishedSearch{inner: inner, rdb: rdb, ttl: ttl, indexID: idx}
}

// NormalizeSearchQueryForCache collapses case and whitespace so equivalent queries share a key.
func NormalizeSearchQueryForCache(q string) string {
	q = strings.TrimSpace(q)
	q = strings.ToLower(q)
	return strings.Join(strings.Fields(q), " ")
}

func publishedSearchCacheKeyMaterial(normQ string, from, size int, highlight bool, indexID string) string {
	return strings.Join([]string{
		publishedSearchFilterVersion,
		"idx=" + indexID,
		"q=" + normQ,
		fmt.Sprintf("from=%d", from),
		fmt.Sprintf("size=%d", size),
		fmt.Sprintf("hl=%t", highlight),
	}, "|")
}

func (c *CachedPublishedSearch) redisKey(normQ string, from, size int, highlight bool) string {
	mat := publishedSearchCacheKeyMaterial(normQ, from, size, highlight, c.indexID)
	sum := sha256.Sum256([]byte(mat))
	return "videosearch:v1:" + hex.EncodeToString(sum[:])
}

// SearchPublishedVideos implements the same contract as [esclient.Client.SearchPublishedVideos].
func (c *CachedPublishedSearch) SearchPublishedVideos(ctx context.Context, q string, from, size int, withHighlight bool) (*esclient.SearchPublishedResult, error) {
	if c == nil || c.inner == nil {
		return nil, fmt.Errorf("cache: nil published search")
	}
	if c.rdb == nil || c.ttl <= 0 {
		return c.inner.SearchPublishedVideos(ctx, q, from, size, withHighlight)
	}

	norm := NormalizeSearchQueryForCache(q)
	key := c.redisKey(norm, from, size, withHighlight)

	getCtx, getSp := tracing.Start(ctx, "redis.videosearch.get",
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "GET"),
		attribute.String("redis.key", key),
	)
	raw, err := c.rdb.Get(getCtx, key).Result()
	if err == nil {
		var res esclient.SearchPublishedResult
		uerr := json.Unmarshal([]byte(raw), &res)
		if uerr == nil {
			getSp.SetAttributes(attribute.String("videosearch.cache", "hit"))
			tracing.Finish(getSp, nil)
			slog.InfoContext(ctx, "video_search",
				"search_cache", "hit",
				"q", q,
				"from", from,
				"size", size,
				"total", res.Total,
				"returned", len(res.Hits),
			)
			return &res, nil
		}
		tracing.Finish(getSp, nil)
		slog.WarnContext(ctx, "video_search_cache_corrupt", "error", uerr)
		delCtx, delSp := tracing.Start(ctx, "redis.videosearch.del",
			attribute.String("db.system", "redis"),
			attribute.String("db.operation", "DEL"),
			attribute.String("redis.key", key),
		)
		delErr := c.rdb.Del(delCtx, key).Err()
		tracing.Finish(delSp, delErr)
	} else if err == redis.Nil {
		getSp.SetAttributes(attribute.String("videosearch.cache", "miss"))
		tracing.Finish(getSp, nil)
	} else {
		tracing.Finish(getSp, err)
		slog.WarnContext(ctx, "video_search_cache_get_failed", "error", err)
	}

	res, err := c.inner.SearchPublishedVideos(ctx, q, from, size, withHighlight)
	if err != nil {
		return nil, err
	}

	if b, merr := json.Marshal(res); merr == nil {
		setCtx, setSp := tracing.Start(ctx, "redis.videosearch.set",
			attribute.String("db.system", "redis"),
			attribute.String("db.operation", "SET"),
			attribute.String("redis.key", key),
		)
		setErr := c.rdb.Set(setCtx, key, b, c.ttl).Err()
		tracing.Finish(setSp, setErr)
		if setErr != nil {
			slog.WarnContext(ctx, "video_search_cache_set_failed", "error", setErr)
		}
	}

	slog.InfoContext(ctx, "video_search",
		"search_cache", "miss",
		"q", q,
		"from", from,
		"size", size,
		"total", res.Total,
		"returned", len(res.Hits),
	)
	return res, nil
}
