package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr string
	// ElasticsearchURL is a comma-separated list of node URLs for the video search index (optional until search is wired).
	ElasticsearchURL         string
	ElasticsearchUsername    string
	ElasticsearchPassword    string
	ElasticsearchIndexVideos string
	AWSRegion                string
	AWSEndpoint              string
	AWSAccessKey             string
	AWSSecretKey             string
	S3RawBucket              string
	S3EncodedBucket          string
	SQSEncodeQueue           string
	SQSMetadataQueue         string
	MongoURI                 string
	MongoDB                  string
	RedisAddr                string
	RedisTTL                 time.Duration
	// SearchCacheTTLSec is Redis TTL for GET /videos/search responses (seconds). 0 disables search caching.
	SearchCacheTTLSec int
	CORSOrigins       []string
	PublicBaseURL     string
	// WebSocketToken optional; non-empty requires ?token= on GET /ws upgrade (see internal/ws).
	WebSocketToken string
	// WSEventChannel: Redis Pub/Sub channel for cross-process WebSocket events (worker → API, multi-pod).
	// Empty = Hub in-process only (worker cannot push to browser WS without this or another bridge).
	WSEventChannel string
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
	esIndex := getenv("ELASTICSEARCH_INDEX_VIDEOS", "videos")
	searchCacheTTL := parseSearchCacheTTLSec(strings.TrimSpace(os.Getenv("REDIS_SEARCH_CACHE_TTL_SEC")))
	return Config{
		HTTPAddr:                 getenv("HTTP_ADDR", ":8080"),
		ElasticsearchURL:         strings.TrimSpace(os.Getenv("ELASTICSEARCH_URL")),
		ElasticsearchUsername:    getenv("ELASTICSEARCH_USERNAME", ""),
		ElasticsearchPassword:    getenv("ELASTICSEARCH_PASSWORD", ""),
		ElasticsearchIndexVideos: esIndex,
		AWSRegion:                getenv("AWS_REGION", "us-east-1"),
		AWSEndpoint:              getenv("AWS_ENDPOINT_URL", "http://localhost:4566"),
		AWSAccessKey:             getenv("AWS_ACCESS_KEY_ID", "test"),
		AWSSecretKey:             getenv("AWS_SECRET_ACCESS_KEY", "test"),
		S3RawBucket:              getenv("S3_RAW_BUCKET", "video-raw"),
		S3EncodedBucket:          getenv("S3_ENCODED_BUCKET", "video-encoded"),
		SQSEncodeQueue:           os.Getenv("SQS_ENCODE_QUEUE_URL"),
		SQSMetadataQueue:         strings.TrimSpace(os.Getenv("SQS_VIDEO_METADATA_QUEUE_URL")),
		MongoURI:                 getenv("MONGODB_URI", "mongodb://localhost:27017"),
		MongoDB:                  getenv("MONGODB_DB", "video_demo"),
		RedisAddr:                getenv("REDIS_ADDR", "localhost:6379"),
		RedisTTL:                 time.Duration(ttlSec) * time.Second,
		SearchCacheTTLSec:        searchCacheTTL,
		CORSOrigins:              cors,
		PublicBaseURL:            strings.TrimRight(getenv("PUBLIC_BASE_URL", "http://localhost:8080"), "/"),
		WebSocketToken:           strings.TrimSpace(os.Getenv("WEBSOCKET_TOKEN")),
		WSEventChannel:           strings.TrimSpace(os.Getenv("WS_EVENT_CHANNEL")),
	}
}

// ElasticsearchAddresses splits ELASTICSEARCH_URL by comma; empty string yields nil.
func (c Config) ElasticsearchAddresses() []string {
	if strings.TrimSpace(c.ElasticsearchURL) == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(c.ElasticsearchURL, ",") {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// parseSearchCacheTTLSec parses REDIS_SEARCH_CACHE_TTL_SEC: empty → 60; invalid/negative → 60; 0 → disabled (no cache).
func parseSearchCacheTTLSec(s string) int {
	if s == "" {
		return 60
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 60
	}
	if n == 0 {
		return 0
	}
	const maxSearchCacheTTL = 3600
	if n > maxSearchCacheTTL {
		return maxSearchCacheTTL
	}
	return n
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
