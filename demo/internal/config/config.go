package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr        string
	AWSRegion       string
	AWSEndpoint     string
	AWSAccessKey    string
	AWSSecretKey    string
	S3RawBucket     string
	S3EncodedBucket string
	SQSEncodeQueue  string
	MongoURI        string
	MongoDB         string
	RedisAddr       string
	RedisTTL        time.Duration
	CORSOrigins     []string
	PublicBaseURL   string
}

func Load() Config {
	ttlSec := parsePositiveIntOr(getenv("REDIS_CACHE_TTL_SEC", ""), 300)
	origins := getenv("CORS_ORIGINS", "http://localhost:5173")
	var cors []string
	for _, o := range strings.Split(origins, ",") {
		if s := strings.TrimSpace(o); s != "" {
			cors = append(cors, s)
		}
	}
	// Non-empty but whitespace-only env (e.g. "   ") parses to zero origins and breaks CORS entirely.
	if len(cors) == 0 {
		cors = []string{"http://localhost:5173", "http://127.0.0.1:5173"}
	}
	return Config{
		HTTPAddr:        getenv("HTTP_ADDR", ":8080"),
		AWSRegion:       getenv("AWS_REGION", "us-east-1"),
		AWSEndpoint:     getenv("AWS_ENDPOINT_URL", "http://localhost:4566"),
		AWSAccessKey:    getenv("AWS_ACCESS_KEY_ID", "test"),
		AWSSecretKey:    getenv("AWS_SECRET_ACCESS_KEY", "test"),
		S3RawBucket:     getenv("S3_RAW_BUCKET", "video-raw"),
		S3EncodedBucket: getenv("S3_ENCODED_BUCKET", "video-encoded"),
		SQSEncodeQueue:  os.Getenv("SQS_ENCODE_QUEUE_URL"),
		MongoURI:        getenv("MONGODB_URI", "mongodb://localhost:27017"),
		MongoDB:         getenv("MONGODB_DB", "video_demo"),
		RedisAddr:       getenv("REDIS_ADDR", "localhost:6379"),
		RedisTTL:        time.Duration(ttlSec) * time.Second,
		CORSOrigins:     cors,
		PublicBaseURL:   strings.TrimRight(getenv("PUBLIC_BASE_URL", "http://localhost:8080"), "/"),
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// parsePositiveIntOr parses s as base-10 int; if empty, invalid, or negative, returns def.
func parsePositiveIntOr(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return def
	}
	return n
}
