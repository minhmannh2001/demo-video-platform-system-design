package ws

import (
	"encoding/json"
	"testing"
)

func TestTopicFromSubscribe_video(t *testing.T) {
	topic, err := TopicFromSubscribe("abc123", "")
	if err != nil {
		t.Fatal(err)
	}
	if topic != "video:abc123" {
		t.Fatalf("topic = %q", topic)
	}
}

func TestTopicFromSubscribe_uploadsChannel_mapsCatalog(t *testing.T) {
	topic, err := TopicFromSubscribe("", ChannelUploads)
	if err != nil {
		t.Fatal(err)
	}
	if topic != TopicCatalog {
		t.Fatalf("topic = %q want %q", topic, TopicCatalog)
	}
}

func TestTopicFromSubscribe_bothSet_invalid(t *testing.T) {
	_, err := TopicFromSubscribe("x", ChannelUploads)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestTopicFromSubscribe_unknownChannel(t *testing.T) {
	_, err := TopicFromSubscribe("", "other")
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsUnknownChannel(err) {
		t.Fatalf("want unknown channel: %v", err)
	}
}

func TestParseClientText_omittedV_defaults1(t *testing.T) {
	m, err := ParseClientText([]byte(`{"type":"ping"}`))
	if err != nil {
		t.Fatal(err)
	}
	if m.V != 1 || m.Type != TypePing {
		t.Fatalf("%+v", m)
	}
}

func TestParseClientText_unsupportedV(t *testing.T) {
	_, err := ParseClientText([]byte(`{"type":"ping","v":2}`))
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsUnsupportedVersion(err) {
		t.Fatalf("got %v", err)
	}
}

func TestServerEnvelopeVideoUpdated_roundTrip(t *testing.T) {
	b, err := ServerEnvelopeVideoUpdated(VideoUpdatedPayload{
		VideoID: "v1", Status: "ready", ManifestURL: "https://x/m.m3u8",
	})
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if string(out["type"]) != `"`+TypeVideoUpdated+`"` {
		t.Fatalf("type = %s", out["type"])
	}
	if string(out["v"]) != "1" {
		t.Fatalf("v = %s", out["v"])
	}
}

func TestServerEnvelopeCatalogInvalidate(t *testing.T) {
	b, err := ServerEnvelopeCatalogInvalidate()
	if err != nil {
		t.Fatal(err)
	}
	var msg struct {
		Type string `json:"type"`
		V    int    `json:"v"`
	}
	if err := json.Unmarshal(b, &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Type != TypeCatalogInvalidate || msg.V != 1 {
		t.Fatalf("%+v", msg)
	}
}
