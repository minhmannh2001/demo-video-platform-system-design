package search

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"video-platform/internal/models"
)

func TestVideoSearchDocFromVideo_JSONMapping(t *testing.T) {
	ts := time.Date(2026, 3, 31, 10, 0, 0, 0, time.UTC)
	ts2 := time.Date(2026, 3, 31, 11, 30, 0, 0, time.UTC)
	v := &models.Video{
		ID:            "507f1f77bcf86cd799439011",
		Title:         "Pasta night",
		Description:   "Homemade sauce and basil.",
		Uploader:      "user-alice",
		Visibility:    models.VisibilityUnlisted,
		RawS3Key:      "videos/507f1f77bcf86cd799439011/original.mp4",
		EncodedPrefix: "videos/507f1f77bcf86cd799439011/hls/",
		Status:        models.StatusReady,
		DurationSec:   120,
		CreatedAt:     ts,
		UpdatedAt:     ts2,
	}

	doc, err := VideoSearchDocFromVideo(v)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}

	wantKeys := map[string]bool{
		"video_id":         true,
		"title":            true,
		"description":      true,
		"owner_id":         true,
		"encoding_status":  true,
		"visibility":       true,
		"created_at":       true,
		"updated_at":       true,
	}
	if len(m) != len(wantKeys) {
		t.Fatalf("key count: got %d want %d, keys=%v", len(m), len(wantKeys), keysOf(m))
	}
	for k := range m {
		if !wantKeys[k] {
			t.Fatalf("unexpected key %q", k)
		}
	}

	assertJSONString(t, m["video_id"], "507f1f77bcf86cd799439011")
	assertJSONString(t, m["title"], "Pasta night")
	assertJSONString(t, m["description"], "Homemade sauce and basil.")
	assertJSONString(t, m["owner_id"], "user-alice")
	assertJSONString(t, m["encoding_status"], models.StatusReady)
	assertJSONString(t, m["visibility"], models.VisibilityUnlisted)
	assertJSONString(t, m["created_at"], "2026-03-31T10:00:00Z")
	assertJSONString(t, m["updated_at"], "2026-03-31T11:30:00Z")

	s := string(raw)
	if strings.Contains(s, "raw_s3") || strings.Contains(s, "encoded_prefix") || strings.Contains(s, "duration") {
		t.Fatalf("JSON must not contain storage or non-index fields: %s", s)
	}
}

func TestVideoSearchDocFromVideo_EffectiveVisibilityDefault(t *testing.T) {
	v := &models.Video{
		ID:          "507f191e810c19729de860ea",
		Title:       "t",
		Description: "",
		Uploader:    "u",
		Status:      models.StatusProcessing,
		CreatedAt:   time.Unix(0, 0).UTC(),
		UpdatedAt:   time.Unix(0, 0).UTC(),
	}
	doc, err := VideoSearchDocFromVideo(v)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Visibility != models.VisibilityPublic {
		t.Fatalf("empty visibility -> public, got %q", doc.Visibility)
	}
}

func TestVideoSearchDocFromVideo_Errors(t *testing.T) {
	if _, err := VideoSearchDocFromVideo(nil); err != ErrNilVideo {
		t.Fatalf("nil: %v", err)
	}
	if _, err := VideoSearchDocFromVideo(&models.Video{Title: "x"}); err != ErrEmptyVideoID {
		t.Fatalf("empty id: %v", err)
	}
}

func keysOf(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func assertJSONString(t *testing.T, raw json.RawMessage, want string) {
	t.Helper()
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("unmarshal %s: %v", raw, err)
	}
	if s != want {
		t.Fatalf("got %q want %q", s, want)
	}
}
