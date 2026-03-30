package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"video-platform/demo/internal/awsclient"
	"video-platform/demo/internal/cache"
	"video-platform/demo/internal/config"
	"video-platform/demo/internal/store"
	"video-platform/demo/internal/tracing"
	"video-platform/demo/internal/worker"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.opentelemetry.io/otel/attribute"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	shutdownTrace, err := tracing.Init(ctx, tracing.InitConfig{
		ServiceName:               "video-worker",
		EnableHTTPInstrumentation: false,
	})
	if err != nil {
		log.Fatalf("tracing init: %v", err)
	}
	defer func() {
		tctx, cancel := context.WithTimeout(context.Background(), tracing.ShutdownTimeout)
		defer cancel()
		if err := shutdownTrace(tctx); err != nil {
			log.Printf("tracing shutdown: %v", err)
		}
	}()

	mongoClient, err := store.Connect(ctx, cfg.MongoURI)
	if err != nil {
		log.Fatalf("mongo connect: %v", err)
	}
	defer func() {
		_ = mongoClient.Disconnect(context.Background())
	}()
	if err := mongoClient.Ping(ctx, readpref.Primary()); err != nil {
		log.Fatalf("mongo ping: %v", err)
	}

	videoStore := store.NewVideoStore(mongoClient.Database(cfg.MongoDB))
	redisCache := cache.New(cfg.RedisAddr, cfg.RedisTTL)

	awsCli, err := awsclient.New(ctx, cfg)
	if err != nil {
		log.Fatalf("aws: %v", err)
	}
	queueURL, err := awsclient.ResolveQueueURL(ctx, awsCli.SQS, cfg)
	if err != nil {
		log.Fatalf("sqs queue: %v", err)
	}

	proc := worker.NewProcessor(worker.Deps{
		S3:            awsCli.S3,
		RawBucket:     cfg.S3RawBucket,
		EncodedBucket: cfg.S3EncodedBucket,
		Store:         videoStore,
		Encoder:       worker.FFmpegEncoder{},
		Cache:         redisCache,
		TempDirParent: os.TempDir(),
	})

	runCtx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	log.Printf("encoder worker polling %s", queueURL)
	for {
		if runCtx.Err() != nil {
			log.Printf("shutdown: %v", runCtx.Err())
			return
		}
		out, err := awsCli.SQS.ReceiveMessage(runCtx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(queueURL),
			MaxNumberOfMessages: 1,
			WaitTimeSeconds:     20,
			VisibilityTimeout:   300,
			MessageAttributeNames: []string{"All"},
		})
		if err != nil {
			if runCtx.Err() != nil {
				return
			}
			log.Printf("receive: %v", err)
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
			msgCtx, span := tracing.Start(msgCtx, "worker.encode_job", spanAttrs...)
			procErr := proc.HandleMessage(msgCtx, body)
			tracing.Finish(span, procErr)
			if procErr != nil {
				log.Printf("job error: %v", procErr)
			}
			_, delErr := awsCli.SQS.DeleteMessage(runCtx, &sqs.DeleteMessageInput{
				QueueUrl:      aws.String(queueURL),
				ReceiptHandle: aws.String(rh),
			})
			if delErr != nil {
				log.Printf("delete message: %v", delErr)
			}
		}
	}
}
