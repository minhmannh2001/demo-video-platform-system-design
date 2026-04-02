// Command search-backfill performs a one-time reindex of all MongoDB videos into Elasticsearch
// (for data created before the metadata-index queue existed).
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"

	"video-platform/internal/applog"
	"video-platform/internal/config"
	"video-platform/internal/models"
	"video-platform/internal/search"
	"video-platform/internal/search/esclient"
	"video-platform/internal/store"

	"go.mongodb.org/mongo-driver/mongo/readpref"
)

func main() {
	applog.Init("search-backfill")

	mongoBatch := flag.Int("mongo-batch", 500, "max videos read from Mongo per in-memory batch")
	bulkSize := flag.Int("bulk", 200, "documents per Elasticsearch _bulk request")
	dryRun := flag.Bool("dry-run", false, "scan Mongo and count only; do not call Elasticsearch")
	skipEnsure := flag.Bool("skip-ensure-index", false, "skip composable template + index creation")
	noRefresh := flag.Bool("no-final-refresh", false, "do not call indices refresh at the end (faster; search may lag briefly)")
	flag.Parse()

	ctx := context.Background()
	cfg := config.Load()

	mongoClient, err := store.Connect(ctx, cfg.MongoURI)
	if err != nil {
		slog.Error("mongo connect failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = mongoClient.Disconnect(shutdownCtx)
	}()

	if err := mongoClient.Ping(ctx, readpref.Primary()); err != nil {
		slog.Error("mongo ping failed", "error", err)
		os.Exit(1)
	}

	videoStore := store.NewVideoStore(mongoClient.Database(cfg.MongoDB))

	var esClient *esclient.Client
	if !*dryRun {
		var err error
		esClient, err = esclient.NewFromAppConfig(cfg)
		if err != nil {
			slog.Error("elasticsearch client failed (set ELASTICSEARCH_URL)", "error", err)
			os.Exit(1)
		}
		if !*skipEnsure {
			if err := esClient.EnsureVideosIndexSetup(ctx); err != nil {
				slog.Error("ensure ES index failed", "error", err)
				os.Exit(1)
			}
			slog.Info("search_backfill_index_ready", "index", cfg.ElasticsearchIndexVideos)
		}
	}

	var (
		scanned int
		skipped int
		bulked  int
	)
	pending := make([]*search.VideoSearchDoc, 0, *bulkSize)

	flush := func() error {
		if *dryRun || len(pending) == 0 {
			pending = pending[:0]
			return nil
		}
		if err := esClient.BulkUpsertSearchDocs(ctx, pending, "false"); err != nil {
			return err
		}
		bulked += len(pending)
		slog.Info("search_backfill_bulk_ok", "batch_docs", len(pending), "total_bulked", bulked)
		pending = pending[:0]
		return nil
	}

	err = videoStore.ForEachVideoBatch(ctx, *mongoBatch, func(c context.Context, batch []models.Video) error {
		for i := range batch {
			scanned++
			doc, derr := search.VideoSearchDocFromVideo(&batch[i])
			if derr != nil {
				skipped++
				slog.Warn("search_backfill_skip_video", "error", derr, "video_id", batch[i].ID)
				continue
			}
			if !*dryRun {
				pending = append(pending, doc)
				if len(pending) >= *bulkSize {
					if err := flush(); err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		slog.Error("search_backfill_failed", "error", err)
		os.Exit(1)
	}
	if err := flush(); err != nil {
		slog.Error("search_backfill_flush_failed", "error", err)
		os.Exit(1)
	}

	if *dryRun {
		slog.Info("search_backfill_dry_run_done", "videos_scanned", scanned, "would_index", scanned-skipped, "skipped", skipped)
		os.Exit(0)
	}

	if !*noRefresh {
		if err := esClient.RefreshIndex(ctx); err != nil {
			slog.Error("search_backfill_refresh_failed", "error", err)
			os.Exit(1)
		}
		slog.Info("search_backfill_index_refreshed", "index", cfg.ElasticsearchIndexVideos)
	}

	slog.Info("search_backfill_done", "videos_scanned", scanned, "skipped", skipped, "documents_bulk_indexed", bulked)
}
