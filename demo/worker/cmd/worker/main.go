package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"video-platform/demo/internal/applog"
	"video-platform/demo/internal/awsclient"
	"video-platform/demo/internal/cache"
	"video-platform/demo/internal/config"
	"video-platform/demo/internal/store"
	"video-platform/demo/internal/tracing"
	"video-platform/demo/internal/videometaqueue"
	"video-platform/demo/internal/worker"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.opentelemetry.io/otel/attribute"
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

	proc := worker.NewProcessor(worker.Deps{
		S3:                 awsCli.S3,
		RawBucket:          cfg.S3RawBucket,
		EncodedBucket:      cfg.S3EncodedBucket,
		Store:              videoStore,
		Encoder:            worker.FFmpegEncoder{},
		Cache:              redisCache,
		TempDirParent:      os.TempDir(),
		MetadataPublisher:  metaPub,
		MetadataQueueURL:   metaQueueURL,
	})

	runCtx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	slog.Info("encoder worker polling", "queue_url", queueURL)
	for {
		if runCtx.Err() != nil {
			slog.Info("shutdown", "reason", runCtx.Err().Error())
			return
		}
		out, err := awsCli.SQS.ReceiveMessage(runCtx, &sqs.ReceiveMessageInput{
			QueueUrl:              aws.String(queueURL),
			MaxNumberOfMessages:   1,
			WaitTimeSeconds:       20,
			VisibilityTimeout:     300,
			MessageAttributeNames: []string{"All"},
		})
		if err != nil {
			if runCtx.Err() != nil {
				return
			}
			slog.Warn("sqs receive failed", "error", err)
			time.Sleep(2 * time.Second)
			continue
		}
		for _, msg := range out.Messages {
			body := aws.ToString(msg.Body)
			rh := aws.ToString(msg.ReceiptHandle)

			msgCtx := tracing.ExtractFromSQSAttributes(runCtx, msg.MessageAttributes)
			var job struct {
				VideoID string `json:"video_id"`
			}
			spanAttrs := []attribute.KeyValue{
				attribute.String("messaging.system", "aws_sqs"),
			}
			if err := json.Unmarshal([]byte(body), &job); err == nil && job.VideoID != "" {
				spanAttrs = append(spanAttrs, attribute.String("video.id", job.VideoID))
			}
			if job.VideoID != "" {
				slog.InfoContext(msgCtx, "sqs_job_received",
					"video_id", job.VideoID,
					"sqs_message_id", aws.ToString(msg.MessageId),
				)
			}
			msgCtx, span := tracing.Start(msgCtx, "worker.encode_job", spanAttrs...)
			procErr := proc.HandleMessage(msgCtx, body)
			tracing.Finish(span, procErr)
			if procErr != nil {
				slog.ErrorContext(msgCtx, "encode_job_failed", "error", procErr, "video_id", job.VideoID)
			}
			_, delErr := awsCli.SQS.DeleteMessage(runCtx, &sqs.DeleteMessageInput{
				QueueUrl:      aws.String(queueURL),
				ReceiptHandle: aws.String(rh),
			})
			if delErr != nil {
				slog.Warn("sqs delete message failed", "error", delErr)
			}
		}
	}
}
