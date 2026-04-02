package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"video-platform/api/internal/handlers"
	"video-platform/internal/applog"
	"video-platform/internal/awsclient"
	"video-platform/internal/cache"
	"video-platform/internal/config"
	"video-platform/internal/search/esclient"
	"video-platform/internal/store"
	"video-platform/internal/tracing"
	"video-platform/internal/videometaqueue"
	"video-platform/internal/ws"
	"video-platform/internal/wsevents"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

func main() {
	applog.Init("video-api")
	cfg := config.Load()
	ctx := context.Background()

	runCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTrace, err := tracing.Init(ctx, tracing.InitConfig{
		ServiceName:               "video-api",
		EnableHTTPInstrumentation: true,
	})
	if err != nil {
		slog.Error("tracing init failed", "error", err)
		os.Exit(1)
	}

	mongoClient, err := store.Connect(ctx, cfg.MongoURI)
	if err != nil {
		slog.Error("mongo connect failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := mongoClient.Disconnect(shutdownCtx); err != nil {
			slog.Warn("mongo disconnect", "error", err)
		}
	}()

	if err := mongoClient.Ping(ctx, readpref.Primary()); err != nil {
		slog.Error("mongo ping failed", "error", err)
		os.Exit(1)
	}

	db := mongoClient.Database(cfg.MongoDB)
	videoStore := store.NewVideoStore(db)

	redisCache := cache.New(cfg.RedisAddr, cfg.RedisTTL)
	if err := redisCache.Ping(ctx); err != nil {
		slog.Error("redis ping failed", "error", err)
		os.Exit(1)
	}

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
	var metaPub videometaqueue.Publisher = videometaqueue.Noop{}
	if metaErr != nil {
		slog.Warn("metadata sqs queue unavailable; search index events disabled", "error", metaErr)
	} else {
		metaPub = videometaqueue.NewSQSPublisher(awsCli.SQS, metaQueueURL)
	}

	var videoSearch handlers.VideoSearch
	if esCli, err := esclient.NewFromAppConfig(cfg); err != nil {
		slog.Warn("elasticsearch unavailable; GET /videos/search returns 503", "error", err)
	} else if cfg.SearchCacheTTLSec > 0 {
		videoSearch = cache.NewCachedPublishedSearch(
			esCli,
			redisCache.Redis(),
			time.Duration(cfg.SearchCacheTTLSec)*time.Second,
			cfg.ElasticsearchIndexVideos,
		)
	} else {
		videoSearch = esCli
	}

	wsSrv := ws.New(ws.Config{
		AllowedOrigins: cfg.CORSOrigins,
		Token:          cfg.WebSocketToken,
	})
	wsBridge := wsevents.NewBridge(wsSrv.Hub(), redisCache.Redis(), cfg.WSEventChannel)
	if cfg.WSEventChannel != "" {
		go func() {
			wsBridge.RunSubscriber(runCtx)
		}()
	}

	h := handlers.New(cfg, awsCli.S3, awsCli.SQS, queueURL, metaQueueURL, videoStore, redisCache, metaPub, videoSearch, wsBridge)

	root := chi.NewRouter()
	root.Use(handlers.RequestLogMiddleware())
	root.Use(handlers.CORSMiddleware(cfg.CORSOrigins))
	root.Get("/ws", wsSrv.ServeHTTP)
	root.Mount("/", h.Routes())

	handler := tracing.WrapHandler(root)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       0,
		WriteTimeout:      0,
	}

	go func() {
		slog.Info("api listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("listen failed", "error", err)
			os.Exit(1)
		}
	}()

	<-runCtx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Warn("server shutdown", "error", err)
	}

	traceCtx, traceCancel := context.WithTimeout(context.Background(), tracing.ShutdownTimeout)
	defer traceCancel()
	if err := shutdownTrace(traceCtx); err != nil {
		slog.Warn("tracing shutdown", "error", err)
	}
}
