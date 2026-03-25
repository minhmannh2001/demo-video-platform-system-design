package streamutil

import (
	"testing"
)

func TestContentTypeByFilename(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"master.m3u8", "application/vnd.apple.mpegurl"},
		{"playlist.M3U8", "application/vnd.apple.mpegurl"},
		{"segment001.ts", "video/mp2t"},
		{"chunk.TS", "video/mp2t"},
		{"file.mp4", "application/octet-stream"},
		{"", "application/octet-stream"},
	}
	for _, tt := range tests {
		got := ContentTypeByFilename(tt.filename)
		if got != tt.want {
			t.Errorf("ContentTypeByFilename(%q) = %q, want %q", tt.filename, got, tt.want)
		}
	}
}

func TestEncodedHLSObjectKey(t *testing.T) {
	valid := "507f1f77bcf86cd799439011"

	t.Run("ok", func(t *testing.T) {
		k, err := EncodedHLSObjectKey(valid, "master.m3u8")
		if err != nil {
			t.Fatal(err)
		}
		want := "videos/507f1f77bcf86cd799439011/hls/master.m3u8"
		if k != want {
			t.Fatalf("got %q want %q", k, want)
		}
	})

	t.Run("invalid_object_id", func(t *testing.T) {
		_, err := EncodedHLSObjectKey("not-hex", "a.m3u8")
		if err == nil {
			t.Fatal("want error")
		}
	})

	t.Run("path_traversal", func(t *testing.T) {
		for _, file := range []string{"../x", "a/../b", "..", "x/../y"} {
			_, err := EncodedHLSObjectKey(valid, file)
			if err == nil {
				t.Fatalf("want error for file %q", file)
			}
		}
	})

	t.Run("empty_relative", func(t *testing.T) {
		_, err := EncodedHLSObjectKey(valid, "")
		if err == nil {
			t.Fatal("want error")
		}
	})

	t.Run("absolute_style", func(t *testing.T) {
		_, err := EncodedHLSObjectKey(valid, "/master.m3u8")
		if err == nil {
			t.Fatal("want error")
		}
	})
}

func TestManifestURL(t *testing.T) {
	valid := "507f1f77bcf86cd799439011"
	u := ManifestURL("http://localhost:8080", valid)
	want := "http://localhost:8080/stream/507f1f77bcf86cd799439011/master.m3u8"
	if u != want {
		t.Fatalf("got %q want %q", u, want)
	}
	u2 := ManifestURL("https://api.example.com/", valid)
	want2 := "https://api.example.com/stream/" + valid + "/master.m3u8"
	if u2 != want2 {
		t.Fatalf("got %q want %q", u2, want2)
	}
}

func TestRawUploadObjectKey(t *testing.T) {
	valid := "507f1f77bcf86cd799439011"
	k, err := RawUploadObjectKey(valid, ".mp4")
	if err != nil {
		t.Fatal(err)
	}
	if k != "videos/507f1f77bcf86cd799439011/original.mp4" {
		t.Fatalf("got %q", k)
	}
	_, err = RawUploadObjectKey("bad", ".mp4")
	if err == nil {
		t.Fatal("want error")
	}
}
