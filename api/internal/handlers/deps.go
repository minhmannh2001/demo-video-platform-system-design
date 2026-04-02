package handlers

import (
	"context"

	"video-platform/internal/models"
	"video-platform/internal/search/esclient"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// VideoSearch runs catalog search in Elasticsearch only (no Mongo on the hot path).
type VideoSearch interface {
	SearchPublishedVideos(ctx context.Context, q string, from, size int, highlight bool) (*esclient.SearchPublishedResult, error)
}

// S3API is satisfied by *s3.Client (Put/Get/List for Warmup).
type S3API interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	ListBuckets(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error)
}

// SQSAPI is satisfied by *sqs.Client.
type SQSAPI interface {
	SendMessage(ctx context.Context, params *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
}

// VideoRepository is satisfied by *store.VideoStore.
type VideoRepository interface {
	Create(ctx context.Context, v *models.Video) error
	GetByID(ctx context.Context, id string) (*models.Video, error)
	List(ctx context.Context, limit int64) ([]models.Video, error)
	UpdateMetadata(ctx context.Context, id, title, description, visibility string) error
	DeleteByID(ctx context.Context, id string) (bool, error)
}

// VideoCacher is satisfied by *cache.VideoCache; may be nil in tests.
type VideoCacher interface {
	Get(ctx context.Context, id string) (*models.Video, error)
	Set(ctx context.Context, v *models.Video) error
	Del(ctx context.Context, id string) error
}
