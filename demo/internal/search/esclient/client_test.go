package esclient

import (
	"testing"

	"video-platform/demo/internal/config"
)

func TestNew_requiresAddresses(t *testing.T) {
	_, err := New(Config{Addresses: nil})
	if err == nil {
		t.Fatal("expected error")
	}
	_, err = New(Config{Addresses: []string{}})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNew_defaultIndexAndRetries(t *testing.T) {
	// No live ES — only checks constructor accepts addresses.
	c, err := New(Config{Addresses: []string{"http://127.0.0.1:59200"}})
	if err != nil {
		t.Fatal(err)
	}
	if c.index != "videos" {
		t.Fatalf("index %q", c.index)
	}
}

func TestNewFromAppConfig_requiresURL(t *testing.T) {
	_, err := NewFromAppConfig(config.Config{})
	if err == nil {
		t.Fatal("expected error when ELASTICSEARCH_URL missing")
	}
}
