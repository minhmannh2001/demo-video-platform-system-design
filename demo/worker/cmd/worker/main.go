package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"video-platform/demo/internal/awsclient"
	"video-platform/demo/internal/cache"
	"video-platform/demo/internal/config"
	"video-platform/demo/internal/store"
	"video-platform/demo/internal/worker"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

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
			if err := proc.HandleMessage(runCtx, body); err != nil {
				log.Printf("job error: %v", err)
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
