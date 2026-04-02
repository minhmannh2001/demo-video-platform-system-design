package worker

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
)

// FFmpegEncoder runs ffmpeg to produce HLS artifacts for adaptive playback (Part 4 — implementation detail).
//
// Per-output behavior:
//  1. Thumbnail — decode input, seek near start, grab exactly one video frame, scale down, write MJPEG as .jpg.
//  2. 360p — H.264 video scaled to 640×360 (letterboxed), AAC audio, MPEG-TS segments + variant playlist prog.m3u8.
//  3. 720p — same as 360p at 1280×720 with higher video bitrate.
//  4. Master — HLS master playlist listing both variant playlists for hls.js / native HLS.
type FFmpegEncoder struct{}

const (
	thumbSeek       = "00:00:00.200" // after keyframe-friendly offset; works for very short clips
	thumbMaxWidth   = 320
	thumbJPEGQ      = 8 // lower = higher quality (MJPEG scale 2–31)
	hlsSegmentSec   = 6
	hlsGOPFrames    = 48
	variant360Name  = "360p"
	variant720Name  = "720p"
	width360        = 640
	height360       = 360
	videoBitrate360 = "900k"
	maxrate360      = "963k"
	bufsize360      = "1400k"
	width720        = 1280
	height720       = 720
	videoBitrate720 = "2500k"
	maxrate720      = "2675k"
	bufsize720      = "3500k"
	audioBitrate    = "128k"
)

// EncodeToHLS transcodes inputPath into multi-bitrate HLS plus a poster image.
//
// Output layout under outputDir:
//
//	thumbnail.jpg
//	360p/prog.m3u8, 360p/seg_*.ts
//	720p/prog.m3u8, 720p/seg_*.ts
//	master.m3u8
func (FFmpegEncoder) EncodeToHLS(ctx context.Context, inputPath, outputDir string) error {
	thumbPath := filepath.Join(outputDir, "thumbnail.jpg")
	if err := extractJPEGThumbnail(ctx, inputPath, thumbPath); err != nil {
		return err
	}
	return encodeTwoVariantHLS(ctx, inputPath, outputDir)
}

// extractJPEGThumbnail: one frame, small width, JPEG compression (libjpeg via mjpeg encoder).
func extractJPEGThumbnail(ctx context.Context, inputPath, thumbPath string) error {
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-hide_banner", "-loglevel", "error", "-y",
		"-ss", thumbSeek,
		"-i", inputPath,
		"-an",
		"-frames:v", "1",
		"-vf", fmt.Sprintf("scale=%d:-2", thumbMaxWidth),
		"-pix_fmt", "yuvj420p",
		"-strict", "-2",
		"-q:v", fmt.Sprint(thumbJPEGQ),
		thumbPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg thumbnail: %w: %s", err, string(out))
	}
	return nil
}

// encodeTwoVariantHLS: single pass, split video to 360p + 720p; duplicate audio per variant; emit master.m3u8.
func encodeTwoVariantHLS(ctx context.Context, inputPath, outputDir string) error {
	// Padded canvas keeps RESOLUTION in master playlist stable for ABR.
	filter := fmt.Sprintf(
		"[0:v]split=2[v360src][v720src];"+
			"[v360src]scale=w=%d:h=%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2[v360];"+
			"[v720src]scale=w=%d:h=%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2[v720]",
		width360, height360, width360, height360,
		width720, height720, width720, height720,
	)

	outPlaylistPattern := filepath.Join(outputDir, "%v", "prog.m3u8")
	segmentPattern := filepath.Join(outputDir, "%v", "seg_%03d.ts")
	master := filepath.Join(outputDir, "master.m3u8")

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-hide_banner", "-loglevel", "error", "-y",
		"-i", inputPath,
		"-filter_complex", filter,
		"-map", "[v360]", "-map", "0:a?",
		"-map", "[v720]", "-map", "0:a?",
		"-c:v", "libx264", "-preset", "veryfast", "-profile:v", "main",
		"-sc_threshold", "0",
		"-g", fmt.Sprint(hlsGOPFrames), "-keyint_min", fmt.Sprint(hlsGOPFrames),
		"-c:a", "aac", "-b:a", audioBitrate, "-ac", "2",
		"-b:v:0", videoBitrate360, "-maxrate:v:0", maxrate360, "-bufsize:v:0", bufsize360,
		"-b:v:1", videoBitrate720, "-maxrate:v:1", maxrate720, "-bufsize:v:1", bufsize720,
		"-hls_time", fmt.Sprint(hlsSegmentSec),
		"-hls_list_size", "0",
		"-hls_flags", "independent_segments",
		"-hls_segment_type", "mpegts",
		"-f", "hls",
		"-master_pl_name", filepath.Base(master),
		"-var_stream_map", fmt.Sprintf("v:0,a:0,name:%s v:1,a:1,name:%s", variant360Name, variant720Name),
		"-hls_segment_filename", segmentPattern,
		outPlaylistPattern,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg hls abr: %w: %s", err, string(out))
	}
	return nil
}
