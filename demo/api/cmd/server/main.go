package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"video-platform/demo/api/internal/handlers"
	"video-platform/demo/internal/awsclient"
	"video-platform/demo/internal/cache"
	"video-platform/demo/internal/config"
	"video-platform/demo/internal/store"
	"video-platform/demo/internal/tracing"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	shutdownTrace, err := tracing.Init(ctx, tracing.InitConfig{
		ServiceName:               "video-api",
		EnableHTTPInstrumentation: true,
	})
	if err != nil {
		log.Fatalf("tracing init: %v", err)
	}

	mongoClient, err := store.Connect(ctx, cfg.MongoURI)
	if err != nil {
		log.Fatalf("mongo connect: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := mongoClient.Disconnect(shutdownCtx); err != nil {
			log.Printf("mongo disconnect: %v", err)
		}
	}()

	if err := mongoClient.Ping(ctx, readpref.Primary()); err != nil {
		log.Fatalf("mongo ping: %v", err)
	}

	db := mongoClient.Database(cfg.MongoDB)
	videoStore := store.NewVideoStore(db)

	redisCache := cache.New(cfg.RedisAddr, cfg.RedisTTL)
	if err := redisCache.Ping(ctx); err != nil {
		log.Fatalf("redis ping: %v", err)
	}

	awsCli, err := awsclient.New(ctx, cfg)
	if err != nil {
		log.Fatalf("aws client: %v", err)
	}
	queueURL, err := awsclient.ResolveQueueURL(ctx, awsCli.SQS, cfg)
	if err != nil {
		log.Fatalf("sqs queue url: %v", err)
	}

	h := handlers.New(cfg, awsCli.S3, awsCli.SQS, queueURL, videoStore, redisCache)

	root := chi.NewRouter()
	root.Use(handlers.CORSMiddleware(cfg.CORSOrigins))
	root.Mount("/", h.Routes())

	handler := tracing.WrapHandler(root)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      0,
	}

	go func() {
		log.Printf("api listening on %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Printf("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown: %v", err)
	}

	traceCtx, traceCancel := context.WithTimeout(context.Background(), tracing.ShutdownTimeout)
	defer traceCancel()
	if err := shutdownTrace(traceCtx); err != nil {
		log.Printf("tracing shutdown: %v", err)
	}
}
