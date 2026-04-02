package awsclient

import (
	"context"
	"testing"

	"video-platform/internal/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type mockSQS struct {
	url    string
	err    error
	called bool
}

func (m *mockSQS) GetQueueUrl(ctx context.Context, in *sqs.GetQueueUrlInput, _ ...func(*sqs.Options)) (*sqs.GetQueueUrlOutput, error) {
	m.called = true
	if m.err != nil {
		return nil, m.err
	}
	return &sqs.GetQueueUrlOutput{QueueUrl: aws.String(m.url)}, nil
}

func TestResolveQueueURL_usesConfigWhenSet(t *testing.T) {
	cfg := config.Config{SQSEncodeQueue: "https://explicit-queue"}
	m := &mockSQS{}
	u, err := ResolveQueueURL(context.Background(), m, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if u != "https://explicit-queue" {
		t.Fatalf("got %q", u)
	}
	if m.called {
		t.Fatal("should not call SQS when URL is in config")
	}
}

func TestResolveQueueURL_fetchesFromSQS(t *testing.T) {
	cfg := config.Config{}
	m := &mockSQS{url: "https://localstack/queue"}
	u, err := ResolveQueueURL(context.Background(), m, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if u != "https://localstack/queue" {
		t.Fatalf("got %q", u)
	}
	if !m.called {
		t.Fatal("expected SQS GetQueueUrl")
	}
}

func TestResolveMetadataQueueURL_usesConfigWhenSet(t *testing.T) {
	cfg := config.Config{SQSMetadataQueue: "https://meta-queue"}
	m := &mockSQS{}
	u, err := ResolveMetadataQueueURL(context.Background(), m, cfg)
	if err != nil || u != "https://meta-queue" || m.called {
		t.Fatalf("got %q err=%v called=%v", u, err, m.called)
	}
}

func TestResolveMetadataQueueURL_fetchesFromSQS(t *testing.T) {
	cfg := config.Config{}
	m := &mockSQS{url: "https://localstack/meta"}
	u, err := ResolveMetadataQueueURL(context.Background(), m, cfg)
	if err != nil || u != "https://localstack/meta" || !m.called {
		t.Fatalf("got %q err=%v called=%v", u, err, m.called)
	}
}
