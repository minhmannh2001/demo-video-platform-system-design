package cache

import (
	"context"
	"testing"
	"time"

	"video-platform/internal/search/esclient"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

type stubSearch struct {
	calls int
	out   *esclient.SearchPublishedResult
	err   error
}

func (s *stubSearch) SearchPublishedVideos(ctx context.Context, q string, from, size int, withHighlight bool) (*esclient.SearchPublishedResult, error) {
	s.calls++
	return s.out, s.err
}

func TestNormalizeSearchQueryForCache(t *testing.T) {
	if got := NormalizeSearchQueryForCache("  Foo\tBAR  "); got != "foo bar" {
		t.Fatalf("got %q", got)
	}
}

func TestCachedPublishedSearch_secondCallHitsRedis(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)

	stub := &stubSearch{
		out: &esclient.SearchPublishedResult{Total: 3, From: 0, Size: 20, Hits: []esclient.SearchPublishedHit{{VideoID: "a"}}},
	}
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	c := NewCachedPublishedSearch(stub, rdb, time.Minute, "videos")
	ctx := context.Background()

	_, err = c.SearchPublishedVideos(ctx, "Hello", 0, 20, false)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.SearchPublishedVideos(ctx, "  hello  ", 0, 20, false)
	if err != nil {
		t.Fatal(err)
	}
	if stub.calls != 1 {
		t.Fatalf("inner calls = %d, want 1 (second request should be cached)", stub.calls)
	}
}

func TestCachedPublishedSearch_noRedisPassthrough(t *testing.T) {
	stub := &stubSearch{
		out: &esclient.SearchPublishedResult{Total: 1},
	}
	c := NewCachedPublishedSearch(stub, nil, time.Minute, "videos")
	ctx := context.Background()
	_, err := c.SearchPublishedVideos(ctx, "x", 0, 10, false)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.SearchPublishedVideos(ctx, "x", 0, 10, false)
	if err != nil {
		t.Fatal(err)
	}
	if stub.calls != 2 {
		t.Fatalf("calls = %d, want 2 without redis", stub.calls)
	}
}
