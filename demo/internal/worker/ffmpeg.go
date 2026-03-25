package worker

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
)

// FFmpegEncoder runs the ffmpeg CLI to produce HLS under outputDir (master.m3u8 + segments).
type FFmpegEncoder struct{}

// EncodeToHLS transcodes inputPath into HLS with H.264/AAC. Requires ffmpeg on PATH.
func (FFmpegEncoder) EncodeToHLS(ctx context.Context, inputPath, outputDir string) error {
	outPlaylist := filepath.Join(outputDir, "master.m3u8")
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-hide_banner", "-loglevel", "error", "-y",
		"-i", inputPath,
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "23",
		"-c:a", "aac", "-b:a", "128k",
		"-hls_time", "6", "-hls_list_size", "0",
		"-f", "hls", outPlaylist,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(out))
	}
	return nil
}
