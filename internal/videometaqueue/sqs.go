package videometaqueue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"video-platform/internal/tracing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// SQSAPI is the subset of the AWS SQS client used for publishing.
type SQSAPI interface {
	SendMessage(ctx context.Context, params *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
}

// SQSPublisher sends JSON events to a single queue with bounded retries.
type SQSPublisher struct {
	client      SQSAPI
	queueURL    string
	maxAttempts int
}

// NewSQSPublisher returns a publisher that retries transient SendMessage failures.
func NewSQSPublisher(client SQSAPI, queueURL string) *SQSPublisher {
	return &SQSPublisher{
		client:      client,
		queueURL:    queueURL,
		maxAttempts: 5,
	}
}

// Publish marshals the event and sends it with trace context in message attributes.
func (p *SQSPublisher) Publish(ctx context.Context, ev Event) error {
	body, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("videometaqueue: marshal: %w", err)
	}
	var lastErr error
	for attempt := 1; attempt <= p.maxAttempts; attempt++ {
		_, err := p.client.SendMessage(ctx, &sqs.SendMessageInput{
			QueueUrl:          aws.String(p.queueURL),
			MessageBody:       aws.String(string(body)),
			MessageAttributes: tracing.InjectIntoSQSAttributes(ctx),
		})
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt < p.maxAttempts {
			wait := time.Duration(50*(1<<uint(attempt-1))) * time.Millisecond
			if wait > 2*time.Second {
				wait = 2 * time.Second
			}
			slog.WarnContext(ctx, "metadata_sqs_send_retry",
				"attempt", attempt,
				"max", p.maxAttempts,
				"error", err,
				"video_id", ev.VideoID,
				"op", ev.Op,
			)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		}
	}
	return fmt.Errorf("videometaqueue: send after %d attempts: %w", p.maxAttempts, lastErr)
}
