package videosearchsync

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"video-platform/internal/models"
	"video-platform/internal/search"
	"video-platform/internal/videometaqueue"
)

// IndexWriter is the ES write surface used by the consumer (implemented by *esclient.Client).
type IndexWriter interface {
	UpsertVideo(ctx context.Context, doc *search.VideoSearchDoc) error
	DeleteVideo(ctx context.Context, videoID string) error
}

// VideoReader loads current metadata from the primary store.
type VideoReader interface {
	GetByID(ctx context.Context, id string) (*models.Video, error)
}

// Consumer applies metadata queue events to Elasticsearch.
type Consumer struct {
	Store   VideoReader
	Index   IndexWriter
	Versions VersionTracker
}

// Handle parses one SQS message body and updates ES. Returns error on transient failure (message should be retried).
func (c *Consumer) Handle(ctx context.Context, body string) error {
	body = strings.TrimSpace(body)
	if body == "" {
		slog.WarnContext(ctx, "videosearchsync_empty_body")
		return nil
	}
	var ev videometaqueue.Event
	if err := json.Unmarshal([]byte(body), &ev); err != nil {
		slog.WarnContext(ctx, "videosearchsync_invalid_json", "error", err)
		return nil
	}
	if ev.VideoID == "" {
		slog.WarnContext(ctx, "videosearchsync_missing_video_id")
		return nil
	}
	if ev.SchemaVersion != 0 && ev.SchemaVersion != videometaqueue.SchemaV1 {
		slog.WarnContext(ctx, "videosearchsync_unknown_schema", "schema_version", ev.SchemaVersion)
		return nil
	}

	switch ev.Op {
	case videometaqueue.OpDeleted:
		return c.handleDelete(ctx, ev)
	case videometaqueue.OpCreated, videometaqueue.OpUpdated:
		return c.handleUpsert(ctx, ev)
	default:
		slog.WarnContext(ctx, "videosearchsync_unknown_op", "op", ev.Op, "video_id", ev.VideoID)
		return nil
	}
}

func (c *Consumer) handleDelete(ctx context.Context, ev videometaqueue.Event) error {
	if err := c.Index.DeleteVideo(ctx, ev.VideoID); err != nil {
		return err
	}
	if c.Versions != nil {
		_ = c.Versions.Clear(ctx, ev.VideoID)
	}
	slog.InfoContext(ctx, "videosearchsync_index_deleted", "video_id", ev.VideoID)
	return nil
}

func (c *Consumer) handleUpsert(ctx context.Context, ev videometaqueue.Event) error {
	if c.Versions != nil {
		ok, err := c.Versions.ShouldApply(ctx, ev.VideoID, ev.CorrelationVersion)
		if err != nil {
			return err
		}
		if !ok {
			slog.InfoContext(ctx, "videosearchsync_skip_stale",
				"video_id", ev.VideoID,
				"correlation_version", ev.CorrelationVersion,
			)
			return nil
		}
	}

	v, err := c.Store.GetByID(ctx, ev.VideoID)
	if err != nil {
		return fmt.Errorf("videosearchsync get video: %w", err)
	}
	if v == nil {
		if err := c.Index.DeleteVideo(ctx, ev.VideoID); err != nil {
			return err
		}
		if c.Versions != nil {
			_ = c.Versions.Clear(ctx, ev.VideoID)
		}
		slog.InfoContext(ctx, "videosearchsync_orphan_removed", "video_id", ev.VideoID)
		return nil
	}

	doc, err := search.VideoSearchDocFromVideo(v)
	if err != nil {
		return err
	}
	if err := c.Index.UpsertVideo(ctx, doc); err != nil {
		return err
	}
	if c.Versions != nil {
		if err := c.Versions.Commit(ctx, ev.VideoID, ev.CorrelationVersion); err != nil {
			return err
		}
	}
	slog.InfoContext(ctx, "videosearchsync_index_upserted",
		"video_id", ev.VideoID,
		"correlation_version", ev.CorrelationVersion,
	)
	return nil
}
