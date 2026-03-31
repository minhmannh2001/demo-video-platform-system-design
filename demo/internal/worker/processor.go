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
	"video-platform/demo/internal/tracing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.opentelemetry.io/otel/attribute"
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
	vid := attribute.String("video.id", id)

	var v *models.Video
	{
		c, sp := tracing.Start(ctx, "mongo.videos.findOne",
			attribute.String("db.system", "mongodb"),
			attribute.String("db.mongodb.collection", "videos"),
			vid,
		)
		var err error
		v, err = p.d.Store.GetByID(c, id)
		tracing.Finish(sp, err)
		if err != nil {
			return err
		}
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
		p.markFailed(ctx, id)
		return fmt.Errorf("download raw: %w", err)
	}

	hlsDir := filepath.Join(workDir, "hls")
	if err := os.MkdirAll(hlsDir, 0o755); err != nil {
		p.markFailed(ctx, id)
		return err
	}

	{
		c, sp := tracing.Start(ctx, "worker.encode_hls",
			vid,
			attribute.String("encoder", "ffmpeg"),
		)
		err := p.d.Encoder.EncodeToHLS(c, inPath, hlsDir)
		tracing.Finish(sp, err)
		if err != nil {
			p.markFailed(ctx, id)
			return fmt.Errorf("encode: %w", err)
		}
	}

	prefix := fmt.Sprintf("videos/%s/hls", id)
	if err := p.uploadHLSDir(ctx, id, hlsDir); err != nil {
		p.markFailed(ctx, id)
		return fmt.Errorf("upload hls: %w", err)
	}

	{
		c, sp := tracing.Start(ctx, "mongo.videos.updateOne",
			attribute.String("db.system", "mongodb"),
			attribute.String("db.mongodb.collection", "videos"),
			attribute.String("worker.mongo.op", "mark_ready"),
			vid,
		)
		err := p.d.Store.MarkReady(c, id, prefix, 0)
		tracing.Finish(sp, err)
		if err != nil {
			return err
		}
	}
	if p.d.Cache != nil {
		c, sp := tracing.Start(ctx, "redis.video.del",
			attribute.String("redis.key", "video:"+id),
			vid,
		)
		err := p.d.Cache.Del(c, id)
		tracing.Finish(sp, err)
		_ = err
	}
	return nil
}

func (p *Processor) markFailed(ctx context.Context, id string) {
	c, sp := tracing.Start(ctx, "mongo.videos.updateOne",
		attribute.String("db.system", "mongodb"),
		attribute.String("db.mongodb.collection", "videos"),
		attribute.String("worker.mongo.op", "mark_failed"),
		attribute.String("video.id", id),
	)
	err := p.d.Store.MarkFailed(c, id)
	tracing.Finish(sp, err)
}

func (p *Processor) downloadRaw(ctx context.Context, rawKey, destPath string) error {
	c, sp := tracing.Start(ctx, "s3.GetObject",
		attribute.String("aws.service", "S3"),
		attribute.String("s3.bucket", p.d.RawBucket),
		attribute.String("s3.key", rawKey),
	)
	out, err := p.d.S3.GetObject(c, &s3.GetObjectInput{
		Bucket: aws.String(p.d.RawBucket),
		Key:    aws.String(rawKey),
	})
	if err != nil {
		tracing.Finish(sp, err)
		return err
	}
	defer out.Body.Close()

	f, err := os.Create(destPath)
	if err != nil {
		tracing.Finish(sp, err)
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, out.Body)
	tracing.Finish(sp, err)
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
		c, sp := tracing.Start(ctx, "s3.PutObject",
			attribute.String("aws.service", "S3"),
			attribute.String("s3.bucket", p.d.EncodedBucket),
			attribute.String("s3.key", key),
			attribute.String("video.id", videoID),
		)
		_, err = p.d.S3.PutObject(c, &s3.PutObjectInput{
			Bucket:      aws.String(p.d.EncodedBucket),
			Key:         aws.String(key),
			Body:        bytes.NewReader(data),
			ContentType: aws.String(ct),
		})
		tracing.Finish(sp, err)
		return err
	})
}
