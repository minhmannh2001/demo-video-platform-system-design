package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"video-platform/internal/search/esclient"
	"video-platform/internal/store"
	"video-platform/internal/tracing"
	"video-platform/internal/videometaqueue"
	"video-platform/internal/videosearchsync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
)

func runSearchIndexConsumer(ctx context.Context, sqsClient *sqs.Client, queueURL string, videoStore *store.VideoStore, es *esclient.Client, rdb *redis.Client) {
	slog.Info("search index consumer polling", "queue_url", queueURL)
	ver := videosearchsync.NewRedisVersionTracker(rdb)
	cons := &videosearchsync.Consumer{
		Store:    videoStore,
		Index:    es,
		Versions: ver,
	}
	for {
		if ctx.Err() != nil {
			slog.Info("search index consumer shutdown", "reason", ctx.Err().Error())
			return
		}
		out, err := sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:              aws.String(queueURL),
			MaxNumberOfMessages:   1,
			WaitTimeSeconds:       20,
			VisibilityTimeout:     120,
			MessageAttributeNames: []string{"All"},
		})
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Warn("metadata sqs receive failed", "error", err)
			time.Sleep(2 * time.Second)
			continue
		}
		for _, msg := range out.Messages {
			body := aws.ToString(msg.Body)
			rh := aws.ToString(msg.ReceiptHandle)

			msgCtx := tracing.ExtractFromSQSAttributes(ctx, msg.MessageAttributes)
			var ev videometaqueue.Event
			_ = json.Unmarshal([]byte(body), &ev)
			spanAttrs := []attribute.KeyValue{attribute.String("messaging.system", "aws_sqs")}
			if ev.VideoID != "" {
				spanAttrs = append(spanAttrs, attribute.String("video.id", ev.VideoID))
			}
			msgCtx, span := tracing.Start(msgCtx, "worker.search_index_job", spanAttrs...)
			handleErr := cons.Handle(msgCtx, body)
			tracing.Finish(span, handleErr)
			if handleErr != nil {
				slog.ErrorContext(msgCtx, "search_index_job_failed",
					"error", handleErr,
					"video_id", ev.VideoID,
					"sqs_message_id", aws.ToString(msg.MessageId),
				)
				continue
			}
			_, delErr := sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
				QueueUrl:      aws.String(queueURL),
				ReceiptHandle: aws.String(rh),
			})
			if delErr != nil {
				slog.Warn("metadata sqs delete message failed", "error", delErr)
			}
		}
	}
}
