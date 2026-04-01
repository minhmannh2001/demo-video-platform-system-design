package esclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"

	"video-platform/demo/internal/config"
	"video-platform/demo/internal/search"
)

// NewFromAppConfig builds a client from [config.Config] (ELASTICSEARCH_* env vars).
// Returns an error if ELASTICSEARCH_URL is unset or whitespace-only.
func NewFromAppConfig(cfg config.Config) (*Client, error) {
	addrs := cfg.ElasticsearchAddresses()
	if len(addrs) == 0 {
		return nil, errors.New("esclient: ELASTICSEARCH_URL is not set")
	}
	return New(Config{
		Addresses:  addrs,
		Username:   cfg.ElasticsearchUsername,
		Password:   cfg.ElasticsearchPassword,
		Index:      cfg.ElasticsearchIndexVideos,
	})
}

// Client indexes and deletes video search documents in Elasticsearch.
type Client struct {
	es    *elasticsearch.Client
	index string
}

// New builds a client for the given addresses. At least one address is required (e.g. http://localhost:9200).
func New(cfg Config) (*Client, error) {
	if len(cfg.Addresses) == 0 {
		return nil, errors.New("esclient: Addresses required")
	}
	idx := strings.TrimSpace(cfg.Index)
	if idx == "" {
		idx = "videos"
	}
	retries := cfg.MaxRetries
	if retries <= 0 {
		retries = 3
	}
	esCfg := elasticsearch.Config{
		Addresses:  cfg.Addresses,
		MaxRetries: retries,
	}
	u := strings.TrimSpace(cfg.Username)
	p := strings.TrimSpace(cfg.Password)
	if u != "" || p != "" {
		esCfg.Username = u
		esCfg.Password = p
	}
	es, err := elasticsearch.NewClient(esCfg)
	if err != nil {
		return nil, fmt.Errorf("esclient: new elasticsearch client: %w", err)
	}
	return &Client{es: es, index: idx}, nil
}

// UpsertVideo indexes or replaces the document; document _id is video_id.
func (c *Client) UpsertVideo(ctx context.Context, doc *search.VideoSearchDoc) error {
	if doc == nil {
		return errors.New("esclient: nil document")
	}
	if doc.VideoID == "" {
		return errors.New("esclient: empty video_id")
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(doc); err != nil {
		return fmt.Errorf("esclient: encode document: %w", err)
	}
	req := esapi.IndexRequest{
		Index:      c.index,
		DocumentID: doc.VideoID,
		Body:       bytes.NewReader(buf.Bytes()),
		Refresh:    "true",
	}
	res, err := req.Do(ctx, c.es)
	if err != nil {
		slog.ErrorContext(ctx, "elasticsearch_index_request_failed",
			"index", c.index,
			"video_id", doc.VideoID,
			"error", err,
		)
		return fmt.Errorf("esclient: index request: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		b, _ := io.ReadAll(res.Body)
		slog.ErrorContext(ctx, "elasticsearch_index_error_response",
			"index", c.index,
			"video_id", doc.VideoID,
			"status", res.StatusCode,
			"body", string(b),
		)
		return fmt.Errorf("esclient: index %s: %s", res.Status(), string(b))
	}
	return nil
}

// DeleteVideo removes the document by video_id. Missing documents are treated as success (idempotent).
func (c *Client) DeleteVideo(ctx context.Context, videoID string) error {
	if videoID == "" {
		return errors.New("esclient: empty video_id")
	}
	req := esapi.DeleteRequest{
		Index:      c.index,
		DocumentID: videoID,
		Refresh:    "true",
	}
	res, err := req.Do(ctx, c.es)
	if err != nil {
		slog.ErrorContext(ctx, "elasticsearch_delete_request_failed",
			"index", c.index,
			"video_id", videoID,
			"error", err,
		)
		return fmt.Errorf("esclient: delete request: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return nil
	}
	if res.IsError() {
		b, _ := io.ReadAll(res.Body)
		slog.ErrorContext(ctx, "elasticsearch_delete_error_response",
			"index", c.index,
			"video_id", videoID,
			"status", res.StatusCode,
			"body", string(b),
		)
		return fmt.Errorf("esclient: delete %s: %s", res.Status(), string(b))
	}
	return nil
}

// GetVideoSource fetches _source for a video id (for tests and diagnostics).
func (c *Client) GetVideoSource(ctx context.Context, videoID string) (json.RawMessage, bool, error) {
	if videoID == "" {
		return nil, false, errors.New("esclient: empty video_id")
	}
	req := esapi.GetRequest{
		Index:      c.index,
		DocumentID: videoID,
	}
	res, err := req.Do(ctx, c.es)
	if err != nil {
		return nil, false, fmt.Errorf("esclient: get request: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return nil, false, nil
	}
	if res.IsError() {
		b, _ := io.ReadAll(res.Body)
		slog.ErrorContext(ctx, "elasticsearch_get_error_response",
			"index", c.index,
			"video_id", videoID,
			"status", res.StatusCode,
			"body", string(b),
		)
		return nil, false, fmt.Errorf("esclient: get %s: %s", res.Status(), string(b))
	}
	var wrap struct {
		Source json.RawMessage `json:"_source"`
	}
	if err := json.NewDecoder(res.Body).Decode(&wrap); err != nil {
		return nil, false, fmt.Errorf("esclient: decode get response: %w", err)
	}
	return wrap.Source, true, nil
}
