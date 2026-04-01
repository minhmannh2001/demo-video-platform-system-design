package esclient

import (
	"strings"
	"testing"
	"time"

	"video-platform/demo/internal/search"
)

func TestBulkNDJSONBody_skipsNilAndEmptyID(t *testing.T) {
	docs := []*search.VideoSearchDoc{
		nil,
		{VideoID: "", Title: "x"},
		{VideoID: "a1", Title: "hello", Description: "d"},
	}
	b, err := bulkNDJSONBody("videos", docs)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 lines (meta+source), got %d: %q", len(lines), string(b))
	}
	if !strings.Contains(lines[0], `"_index":"videos"`) || !strings.Contains(lines[0], `"_id":"a1"`) {
		t.Fatalf("bad meta line: %s", lines[0])
	}
	if !strings.Contains(lines[1], `"video_id":"a1"`) || !strings.Contains(lines[1], `"title":"hello"`) {
		t.Fatalf("bad source line: %s", lines[1])
	}
}

func TestBulkNDJSONBody_allSkippedReturnsEmpty(t *testing.T) {
	b, err := bulkNDJSONBody("videos", []*search.VideoSearchDoc{{VideoID: ""}})
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 0 {
		t.Fatalf("want empty body, got %q", b)
	}
}

func TestBulkNDJSONBody_timeFieldsJSON(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	docs := []*search.VideoSearchDoc{{
		VideoID: "x", Title: "t", EncodingStatus: "ready", Visibility: "public",
		CreatedAt: ts, UpdatedAt: ts,
	}}
	b, err := bulkNDJSONBody("idx", docs)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "2026-01-02T03:04:05") {
		t.Fatalf("expected RFC3339 time in JSON: %s", string(b))
	}
}
