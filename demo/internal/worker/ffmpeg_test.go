package worker

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFFmpegEncoder_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skip ffmpeg integration in -short")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not on PATH")
	}

	dir := t.TempDir()
	inPath := filepath.Join(dir, "in.mp4")
	// Minimal valid MP4 (empty moov) is fragile; generate 1s silence + black with ffmpeg.
	gen := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "color=c=black:s=320x240:d=1",
		"-f", "lavfi", "-i", "anullsrc=r=44100:cl=mono",
		"-shortest", "-c:v", "libx264", "-c:a", "aac", inPath)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("gen input: %v: %s", err, out)
	}

	outDir := filepath.Join(dir, "hls")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var enc FFmpegEncoder
	if err := enc.EncodeToHLS(context.Background(), inPath, outDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "master.m3u8")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "thumbnail.jpg")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "360p", "prog.m3u8")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "720p", "prog.m3u8")); err != nil {
		t.Fatal(err)
	}
	master, err := os.ReadFile(filepath.Join(outDir, "master.m3u8"))
	if err != nil {
		t.Fatal(err)
	}
	ms := string(master)
	if !strings.Contains(ms, "360p/prog.m3u8") || !strings.Contains(ms, "720p/prog.m3u8") {
		t.Fatalf("master should reference both variant playlists:\n%s", ms)
	}
}
