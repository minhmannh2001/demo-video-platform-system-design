package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"video-platform/internal/applog"
	"video-platform/internal/awsclient"
	"video-platform/internal/cache"
	"video-platform/internal/config"
	"video-platform/internal/search/esclient"
	"video-platform/internal/store"
	"video-platform/internal/tracing"
	"video-platform/internal/videometaqueue"
	"video-platform/internal/worker"
	"video-platform/internal/wsevents"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

func main() {
	applog.Init("video-worker")
	cfg := config.Load()
	ctx := context.Background()

	shutdownTrace, err := tracing.Init(ctx, tracing.InitConfig{
		ServiceName:               "video-worker",
		EnableHTTPInstrumentation: false,
	})
	if err != nil {
		slog.Error("tracing init failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		tctx, cancel := context.WithTimeout(context.Background(), tracing.ShutdownTimeout)
		defer cancel()
		if err := shutdownTrace(tctx); err != nil {
			slog.Warn("tracing shutdown", "error", err)
		}
	}()

	mongoClient, err := store.Connect(ctx, cfg.MongoURI)
	if err != nil {
		slog.Error("mongo connect failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		_ = mongoClient.Disconnect(context.Background())
	}()
	if err := mongoClient.Ping(ctx, readpref.Primary()); err != nil {
		slog.Error("mongo ping failed", "error", err)
		os.Exit(1)
	}

	videoStore := store.NewVideoStore(mongoClient.Database(cfg.MongoDB))
	redisCache := cache.New(cfg.RedisAddr, cfg.RedisTTL)

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	defer func() { _ = rdb.Close() }()

	awsCli, err := awsclient.New(ctx, cfg)
	if err != nil {
		slog.Error("aws client init failed", "error", err)
		os.Exit(1)
	}
	queueURL, err := awsclient.ResolveQueueURL(ctx, awsCli.SQS, cfg)
	if err != nil {
		slog.Error("sqs queue url resolve failed", "error", err)
		os.Exit(1)
	}

	metaQueueURL, metaErr := awsclient.ResolveMetadataQueueURL(ctx, awsCli.SQS, cfg)
	var metaPub videometaqueue.Publisher
	if metaErr != nil {
		slog.Warn("metadata sqs queue unavailable; worker will not emit search index events", "error", metaErr)
	} else {
		metaPub = videometaqueue.NewSQSPublisher(awsCli.SQS, metaQueueURL)
	}

	var esClient *esclient.Client
	esClient, esErr := esclient.NewFromAppConfig(cfg)
	if esErr != nil {
		slog.Warn("elasticsearch client unavailable; search index consumer disabled", "error", esErr)
	} else {
		if err := esClient.EnsureVideosIndexSetup(ctx); err != nil {
			slog.Warn("elasticsearch index bootstrap", "error", err)
		}
	}

	var wsBridge *wsevents.Bridge
	if ch := cfg.WSEventChannel; ch != "" {
		wsBridge = wsevents.NewBridge(nil, rdb, ch)
	}

	proc := worker.NewProcessor(worker.Deps{
		S3:                awsCli.S3,
		RawBucket:         cfg.S3RawBucket,
		EncodedBucket:     cfg.S3EncodedBucket,
		Store:             videoStore,
		Encoder:           worker.FFmpegEncoder{},
		Cache:             redisCache,
		TempDirParent:     os.TempDir(),
		MetadataPublisher: metaPub,
		MetadataQueueURL:  metaQueueURL,
		PublicBaseURL:     cfg.PublicBaseURL,
		Realtime:          wsBridge,
	})

	runCtx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		runEncodeWorker(runCtx, awsCli.SQS, queueURL, proc)
	}()

	if esClient != nil && metaErr == nil && metaQueueURL != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runSearchIndexConsumer(runCtx, awsCli.SQS, metaQueueURL, videoStore, esClient, rdb)
		}()
	}

	wg.Wait()
	slog.Info("worker processes stopped")
}
