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
	"video-platform/demo/internal/tracing"

	"go.opentelemetry.io/otel/attribute"
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
	ctx, sp := tracing.Start(ctx, "elasticsearch.index",
		attribute.String("db.system", "elasticsearch"),
		attribute.String("elasticsearch.operation", "index"),
		attribute.String("elasticsearch.index", c.index),
		attribute.String("video.id", doc.VideoID),
	)
	var err error
	defer func() { tracing.Finish(sp, err) }()

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
		err = fmt.Errorf("esclient: index request: %w", err)
		return err
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
		err = fmt.Errorf("esclient: index %s: %s", res.Status(), string(b))
		return err
	}
	return nil
}

// DeleteVideo removes the document by video_id. Missing documents are treated as success (idempotent).
func (c *Client) DeleteVideo(ctx context.Context, videoID string) error {
	if videoID == "" {
		return errors.New("esclient: empty video_id")
	}
	ctx, sp := tracing.Start(ctx, "elasticsearch.delete",
		attribute.String("db.system", "elasticsearch"),
		attribute.String("elasticsearch.operation", "delete"),
		attribute.String("elasticsearch.index", c.index),
		attribute.String("video.id", videoID),
	)
	var err error
	defer func() { tracing.Finish(sp, err) }()

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
		err = fmt.Errorf("esclient: delete request: %w", err)
		return err
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
		err = fmt.Errorf("esclient: delete %s: %s", res.Status(), string(b))
		return err
	}
	return nil
}

// GetVideoSource fetches _source for a video id (for tests and diagnostics).
func (c *Client) GetVideoSource(ctx context.Context, videoID string) (_ json.RawMessage, _ bool, err error) {
	if videoID == "" {
		return nil, false, errors.New("esclient: empty video_id")
	}
	ctx, sp := tracing.Start(ctx, "elasticsearch.get",
		attribute.String("db.system", "elasticsearch"),
		attribute.String("elasticsearch.operation", "get"),
		attribute.String("elasticsearch.index", c.index),
		attribute.String("video.id", videoID),
	)
	defer func() { tracing.Finish(sp, err) }()

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
		err = fmt.Errorf("esclient: get %s: %s", res.Status(), string(b))
		return nil, false, err
	}
	var wrap struct {
		Source json.RawMessage `json:"_source"`
	}
	if decErr := json.NewDecoder(res.Body).Decode(&wrap); decErr != nil {
		err = fmt.Errorf("esclient: decode get response: %w", decErr)
		return nil, false, err
	}
	return wrap.Source, true, nil
}

// RefreshIndex makes recent writes visible for search (POST indices/_refresh).
func (c *Client) RefreshIndex(ctx context.Context) error {
	res, err := esapi.IndicesRefreshRequest{Index: []string{c.index}}.Do(ctx, c.es)
	if err != nil {
		return fmt.Errorf("esclient: refresh index: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("esclient: refresh %s: %s", res.Status(), string(b))
	}
	return nil
}
