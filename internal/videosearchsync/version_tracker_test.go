package videosearchsync

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisVersionTracker_ordering(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	vt := NewRedisVersionTracker(rdb)
	ctx := context.Background()
	vid := "507f1f77bcf86cd799439011"

	ok, err := vt.ShouldApply(ctx, vid, 10)
	if err != nil || !ok {
		t.Fatalf("first apply: ok=%v err=%v", ok, err)
	}
	if err := vt.Commit(ctx, vid, 10); err != nil {
		t.Fatal(err)
	}
	ok, err = vt.ShouldApply(ctx, vid, 10)
	if err != nil || ok {
		t.Fatalf("duplicate same version should skip: ok=%v", ok)
	}
	ok, err = vt.ShouldApply(ctx, vid, 5)
	if err != nil || ok {
		t.Fatalf("stale should skip: ok=%v", ok)
	}
	ok, err = vt.ShouldApply(ctx, vid, 20)
	if err != nil || !ok {
		t.Fatalf("newer should apply: ok=%v", ok)
	}
	if err := vt.Clear(ctx, vid); err != nil {
		t.Fatal(err)
	}
	ok, err = vt.ShouldApply(ctx, vid, 10)
	if err != nil || !ok {
		t.Fatalf("after clear should apply: ok=%v", ok)
	}
}
