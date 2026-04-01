package awsclient

import (
	"context"
	"fmt"

	"video-platform/demo/internal/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type Clients struct {
	S3  *s3.Client
	SQS *sqs.Client
}

func New(ctx context.Context, cfg config.Config) (*Clients, error) {
	resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, _ ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL:           cfg.AWSEndpoint,
			SigningRegion: cfg.AWSRegion,
		}, nil
	})
	awsCfg := aws.Config{
		Region:                      cfg.AWSRegion,
		EndpointResolverWithOptions: resolver,
		Credentials:                 credentials.NewStaticCredentialsProvider(cfg.AWSAccessKey, cfg.AWSSecretKey, ""),
	}
	return &Clients{
		S3:  s3.NewFromConfig(awsCfg, func(o *s3.Options) { o.UsePathStyle = true }),
		SQS: sqs.NewFromConfig(awsCfg),
	}, nil
}

// SQSQueueURLResolver is satisfied by *sqs.Client.
type SQSQueueURLResolver interface {
	GetQueueUrl(ctx context.Context, params *sqs.GetQueueUrlInput, optFns ...func(*sqs.Options)) (*sqs.GetQueueUrlOutput, error)
}

func ResolveQueueURL(ctx context.Context, sqsClient SQSQueueURLResolver, cfg config.Config) (string, error) {
	if cfg.SQSEncodeQueue != "" {
		return cfg.SQSEncodeQueue, nil
	}
	out, err := sqsClient.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{QueueName: aws.String("video-encode-jobs")})
	if err != nil {
		return "", fmt.Errorf("get queue url: %w", err)
	}
	return aws.ToString(out.QueueUrl), nil
}

// ResolveMetadataQueueURL returns the URL for search-index metadata events.
// If SQS_VIDEO_METADATA_QUEUE_URL is set in config, it wins; otherwise resolves by queue name video-metadata-index.
func ResolveMetadataQueueURL(ctx context.Context, sqsClient SQSQueueURLResolver, cfg config.Config) (string, error) {
	if cfg.SQSMetadataQueue != "" {
		return cfg.SQSMetadataQueue, nil
	}
	out, err := sqsClient.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{QueueName: aws.String("video-metadata-index")})
	if err != nil {
		return "", fmt.Errorf("get metadata queue url: %w", err)
	}
	return aws.ToString(out.QueueUrl), nil
}
