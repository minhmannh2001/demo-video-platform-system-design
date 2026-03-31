package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"video-platform/demo/api/internal/handlers"
	"video-platform/demo/internal/applog"
	"video-platform/demo/internal/awsclient"
	"video-platform/demo/internal/cache"
	"video-platform/demo/internal/config"
	"video-platform/demo/internal/store"
	"video-platform/demo/internal/tracing"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

func main() {
	applog.Init("video-api")
	cfg := config.Load()
	ctx := context.Background()

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

	h := handlers.New(cfg, awsCli.S3, awsCli.SQS, queueURL, videoStore, redisCache)

	root := chi.NewRouter()
	root.Use(handlers.RequestLogMiddleware())
	root.Use(handlers.CORSMiddleware(cfg.CORSOrigins))
	root.Mount("/", h.Routes())

	handler := tracing.WrapHandler(root)

	// ReadTimeout applies to the entire request including multipart body. A fixed
	// short limit (e.g. 60s) breaks large/slow uploads and surfaces as net.OpError
	// "read" on TCP. Use 0 for no read deadline on the body (typical for upload APIs).
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

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
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
