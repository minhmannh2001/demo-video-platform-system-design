package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"video-platform/demo/internal/models"
	"video-platform/demo/internal/streamutil"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	ErrInvalidMessage = errors.New("worker: invalid sqs message")
	ErrVideoNotFound  = errors.New("worker: video not found in database")
)

// S3GetPut is satisfied by *s3.Client.
type S3GetPut interface {
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// VideoStore is the subset of persistence used by the encoder worker.
type VideoStore interface {
	GetByID(ctx context.Context, id string) (*models.Video, error)
	MarkReady(ctx context.Context, id, encodedPrefix string, durationSec int) error
	MarkFailed(ctx context.Context, id string) error
}

// Encoder produces HLS files under outputDir (e.g. master.m3u8 and .ts segments).
type Encoder interface {
	EncodeToHLS(ctx context.Context, inputPath, outputDir string) error
}

// CacheInvalidator drops cached metadata after a successful encode.
type CacheInvalidator interface {
	Del(ctx context.Context, id string) error
}

// Deps bundles worker dependencies (injectable for tests).
type Deps struct {
	S3              S3GetPut
	RawBucket       string
	EncodedBucket   string
	Store           VideoStore
	Encoder         Encoder
	Cache           CacheInvalidator
	TempDirParent   string
}

// Processor handles one encode job per HandleMessage call.
type Processor struct {
	d Deps
}

func NewProcessor(d Deps) *Processor {
	if d.TempDirParent == "" {
		d.TempDirParent = os.TempDir()
	}
	return &Processor{d: d}
}

type jobBody struct {
	VideoID string `json:"video_id"`
}

func parseJobBody(body string) (jobBody, error) {
	var j jobBody
	if err := json.Unmarshal([]byte(body), &j); err != nil {
		return jobBody{}, fmt.Errorf("%w: %v", ErrInvalidMessage, err)
	}
	if strings.TrimSpace(j.VideoID) == "" {
		return jobBody{}, ErrInvalidMessage
	}
	return j, nil
}

// HandleMessage decodes SQS JSON {"video_id":"..."} and runs the encode pipeline.
func (p *Processor) HandleMessage(ctx context.Context, body string) error {
	j, err := parseJobBody(body)
	if err != nil {
		return err
	}
	return p.processVideo(ctx, j.VideoID)
}

func (p *Processor) processVideo(ctx context.Context, id string) error {
	v, err := p.d.Store.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if v == nil {
		return ErrVideoNotFound
	}

	workDir, err := os.MkdirTemp(p.d.TempDirParent, "video-enc-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(workDir) }()

	inPath := filepath.Join(workDir, "input"+filepath.Ext(v.RawS3Key))
	if err := p.downloadRaw(ctx, v.RawS3Key, inPath); err != nil {
		_ = p.d.Store.MarkFailed(ctx, id)
		return fmt.Errorf("download raw: %w", err)
	}

	hlsDir := filepath.Join(workDir, "hls")
	if err := os.MkdirAll(hlsDir, 0o755); err != nil {
		_ = p.d.Store.MarkFailed(ctx, id)
		return err
	}

	if err := p.d.Encoder.EncodeToHLS(ctx, inPath, hlsDir); err != nil {
		_ = p.d.Store.MarkFailed(ctx, id)
		return fmt.Errorf("encode: %w", err)
	}

	prefix := fmt.Sprintf("videos/%s/hls", id)
	if err := p.uploadHLSDir(ctx, id, hlsDir); err != nil {
		_ = p.d.Store.MarkFailed(ctx, id)
		return fmt.Errorf("upload hls: %w", err)
	}

	if err := p.d.Store.MarkReady(ctx, id, prefix, 0); err != nil {
		return err
	}
	if p.d.Cache != nil {
		_ = p.d.Cache.Del(ctx, id)
	}
	return nil
}

func (p *Processor) downloadRaw(ctx context.Context, rawKey, destPath string) error {
	out, err := p.d.S3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(p.d.RawBucket),
		Key:    aws.String(rawKey),
	})
	if err != nil {
		return err
	}
	defer out.Body.Close()

	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, out.Body)
	return err
}

func (p *Processor) uploadHLSDir(ctx context.Context, videoID, hlsDir string) error {
	return filepath.WalkDir(hlsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(hlsDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		key, err := streamutil.EncodedHLSObjectKey(videoID, rel)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		ct := streamutil.ContentTypeByFilename(rel)
		_, err = p.d.S3.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(p.d.EncodedBucket),
			Key:         aws.String(key),
			Body:        bytes.NewReader(data),
			ContentType: aws.String(ct),
		})
		return err
	})
}
