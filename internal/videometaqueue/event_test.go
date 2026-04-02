package videometaqueue

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewEvent_JSON(t *testing.T) {
	ts := time.Date(2026, 4, 1, 15, 30, 0, 123456789, time.UTC)
	ev := NewEvent("507f1f77bcf86cd799439011", OpCreated, ts)
	b, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if int(m["schema_version"].(float64)) != SchemaV1 {
		t.Fatalf("schema_version: %v", m["schema_version"])
	}
	if m["video_id"] != ev.VideoID || m["op"] != OpCreated {
		t.Fatalf("%v", m)
	}
	if ev.CorrelationVersion != ts.UnixNano() {
		t.Fatalf("correlation: %d", ev.CorrelationVersion)
	}
}
