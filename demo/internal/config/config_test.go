package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad_defaults(t *testing.T) {
	clearDemoEnv(t)
	// All demo env vars unset → defaults
	c := Load()
	if c.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr: got %q want %q", c.HTTPAddr, ":8080")
	}
	if c.AWSRegion != "us-east-1" {
		t.Fatalf("AWSRegion: got %q", c.AWSRegion)
	}
	if c.AWSEndpoint != "http://localhost:4566" {
		t.Fatalf("AWSEndpoint: got %q", c.AWSEndpoint)
	}
	if c.S3RawBucket != "video-raw" || c.S3EncodedBucket != "video-encoded" {
		t.Fatalf("S3 buckets: raw=%q encoded=%q", c.S3RawBucket, c.S3EncodedBucket)
	}
	if c.MongoURI != "mongodb://localhost:27017" || c.MongoDB != "video_demo" {
		t.Fatalf("Mongo: uri=%q db=%q", c.MongoURI, c.MongoDB)
	}
	if c.RedisAddr != "localhost:6379" {
		t.Fatalf("RedisAddr: got %q", c.RedisAddr)
	}
	if c.RedisTTL != 300*time.Second {
		t.Fatalf("RedisTTL: got %v want 300s", c.RedisTTL)
	}
	if len(c.CORSOrigins) != 1 || c.CORSOrigins[0] != "http://localhost:5173" {
		t.Fatalf("CORSOrigins: got %#v", c.CORSOrigins)
	}
	if c.PublicBaseURL != "http://localhost:8080" {
		t.Fatalf("PublicBaseURL: got %q", c.PublicBaseURL)
	}
	if c.SQSEncodeQueue != "" {
		t.Fatalf("SQSEncodeQueue: want empty when unset, got %q", c.SQSEncodeQueue)
	}
}

func TestLoad_overrides(t *testing.T) {
	clearDemoEnv(t)
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("AWS_REGION", "ap-southeast-1")
	t.Setenv("AWS_ENDPOINT_URL", "http://example:4566")
	t.Setenv("AWS_ACCESS_KEY_ID", "ak")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "sk")
	t.Setenv("S3_RAW_BUCKET", "raw-b")
	t.Setenv("S3_ENCODED_BUCKET", "enc-b")
	t.Setenv("SQS_ENCODE_QUEUE_URL", "http://sqs/queue")
	t.Setenv("MONGODB_URI", "mongodb://mongo:27017")
	t.Setenv("MONGODB_DB", "app")
	t.Setenv("REDIS_ADDR", "redis:6379")
	t.Setenv("REDIS_CACHE_TTL_SEC", "120")
	t.Setenv("CORS_ORIGINS", "http://a:1, http://b:2 ")
	t.Setenv("PUBLIC_BASE_URL", "https://api.example.com/")

	c := Load()
	if c.HTTPAddr != ":9090" {
		t.Fatalf("HTTPAddr: %q", c.HTTPAddr)
	}
	if c.AWSRegion != "ap-southeast-1" || c.AWSEndpoint != "http://example:4566" {
		t.Fatalf("AWS: region=%q endpoint=%q", c.AWSRegion, c.AWSEndpoint)
	}
	if c.AWSAccessKey != "ak" || c.AWSSecretKey != "sk" {
		t.Fatalf("credentials mismatch")
	}
	if c.S3RawBucket != "raw-b" || c.S3EncodedBucket != "enc-b" {
		t.Fatalf("S3 buckets")
	}
	if c.SQSEncodeQueue != "http://sqs/queue" {
		t.Fatalf("SQS: %q", c.SQSEncodeQueue)
	}
	if c.MongoURI != "mongodb://mongo:27017" || c.MongoDB != "app" {
		t.Fatalf("Mongo")
	}
	if c.RedisAddr != "redis:6379" || c.RedisTTL != 120*time.Second {
		t.Fatalf("Redis: addr=%q ttl=%v", c.RedisAddr, c.RedisTTL)
	}
	wantCORS := []string{"http://a:1", "http://b:2"}
	if len(c.CORSOrigins) != len(wantCORS) {
		t.Fatalf("CORS len: got %d", len(c.CORSOrigins))
	}
	for i, w := range wantCORS {
		if c.CORSOrigins[i] != w {
			t.Fatalf("CORS[%d]: got %q want %q", i, c.CORSOrigins[i], w)
		}
	}
	if c.PublicBaseURL != "https://api.example.com" {
		t.Fatalf("PublicBaseURL should trim trailing slash: got %q", c.PublicBaseURL)
	}
}

func TestLoad_cors_whitespaceOnlyFallsBackToDevDefaults(t *testing.T) {
	clearDemoEnv(t)
	t.Setenv("CORS_ORIGINS", "  ,  , ")
	c := Load()
	if len(c.CORSOrigins) != 2 {
		t.Fatalf("CORSOrigins: got %#v want len 2", c.CORSOrigins)
	}
}

func TestLoad_redisTTL_invalidFallsBackToDefault(t *testing.T) {
	clearDemoEnv(t)
	t.Setenv("REDIS_CACHE_TTL_SEC", "not-a-number")
	c := Load()
	if c.RedisTTL != 300*time.Second {
		t.Fatalf("RedisTTL: got %v want 300s", c.RedisTTL)
	}
	t.Setenv("REDIS_CACHE_TTL_SEC", "-1")
	c = Load()
	if c.RedisTTL != 300*time.Second {
		t.Fatalf("RedisTTL negative: got %v want 300s", c.RedisTTL)
	}
}

// clearDemoEnv unsets vars Load() reads so tests don't inherit shell env.
func clearDemoEnv(t *testing.T) {
	t.Helper()
	keys := []string{
		"HTTP_ADDR", "AWS_REGION", "AWS_ENDPOINT_URL", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY",
		"S3_RAW_BUCKET", "S3_ENCODED_BUCKET", "SQS_ENCODE_QUEUE_URL",
		"MONGODB_URI", "MONGODB_DB", "REDIS_ADDR", "REDIS_CACHE_TTL_SEC",
		"CORS_ORIGINS", "PUBLIC_BASE_URL",
	}
	for _, k := range keys {
		t.Cleanup(func() {
			_ = os.Unsetenv(k)
		})
		_ = os.Unsetenv(k)
	}
}
