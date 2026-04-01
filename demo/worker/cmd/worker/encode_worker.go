package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"video-platform/demo/internal/tracing"
	"video-platform/demo/internal/worker"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"go.opentelemetry.io/otel/attribute"
)

func runEncodeWorker(ctx context.Context, sqsClient *sqs.Client, queueURL string, proc *worker.Processor) {
	slog.Info("encoder worker polling", "queue_url", queueURL)
	for {
		if ctx.Err() != nil {
			slog.Info("encoder worker shutdown", "reason", ctx.Err().Error())
			return
		}
		out, err := sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:              aws.String(queueURL),
			MaxNumberOfMessages:   1,
			WaitTimeSeconds:       20,
			VisibilityTimeout:     300,
			MessageAttributeNames: []string{"All"},
		})
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Warn("sqs receive failed", "error", err)
			time.Sleep(2 * time.Second)
			continue
		}
		for _, msg := range out.Messages {
			body := aws.ToString(msg.Body)
			rh := aws.ToString(msg.ReceiptHandle)

			msgCtx := tracing.ExtractFromSQSAttributes(ctx, msg.MessageAttributes)
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
			_, delErr := sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
				QueueUrl:      aws.String(queueURL),
				ReceiptHandle: aws.String(rh),
			})
			if delErr != nil {
				slog.Warn("sqs delete message failed", "error", delErr)
			}
		}
	}
}
