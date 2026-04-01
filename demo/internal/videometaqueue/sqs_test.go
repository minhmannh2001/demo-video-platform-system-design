package videometaqueue

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type mockSQS struct {
	call int
}

func (m *mockSQS) SendMessage(ctx context.Context, in *sqs.SendMessageInput, _ ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	m.call++
	if m.call <= 2 {
		return nil, errors.New("throttle")
	}
	return &sqs.SendMessageOutput{MessageId: aws.String("mid")}, nil
}

func TestSQSPublisher_retriesThenSucceeds(t *testing.T) {
	m := &mockSQS{}
	p := &SQSPublisher{client: m, queueURL: "http://q", maxAttempts: 5}
	err := p.Publish(context.Background(), NewEvent("id1", OpUpdated, time.Unix(1, 0).UTC()))
	if err != nil {
		t.Fatal(err)
	}
	if m.call != 3 {
		t.Fatalf("calls=%d want 3", m.call)
	}
}
