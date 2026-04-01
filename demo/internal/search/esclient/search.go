package esclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"

	"github.com/elastic/go-elasticsearch/v8/esapi"
)

const (
	defaultSearchSize = 20
	maxSearchSize     = 50
)

// SearchPublishedHit is one row from a catalog search (Elasticsearch only).
type SearchPublishedHit struct {
	VideoID    string              `json:"video_id"`
	Score      *float64            `json:"score,omitempty"`
	Highlights map[string][]string `json:"highlights,omitempty"`
}

// SearchPublishedResult is the read-model for GET /videos/search.
type SearchPublishedResult struct {
	Total int64                `json:"total"`
	From  int                  `json:"from"`
	Size  int                  `json:"size"`
	Hits  []SearchPublishedHit `json:"hits"`
}

// SearchPublishedVideos runs multi_match on title (boosted) and description, filtered to
// public visibility and ready encoding status. Does not access MongoDB.
func (c *Client) SearchPublishedVideos(ctx context.Context, q string, from, size int, withHighlight bool) (*SearchPublishedResult, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, fmt.Errorf("esclient: empty search query")
	}
	if from < 0 {
		from = 0
	}
	if size <= 0 {
		size = defaultSearchSize
	}
	if size > maxSearchSize {
		size = maxSearchSize
	}

	body := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": map[string]interface{}{
					"multi_match": map[string]interface{}{
						"query":  q,
						"fields": []string{"title^2", "description"},
						"type":   "best_fields",
					},
				},
				"filter": []interface{}{
					map[string]interface{}{"term": map[string]interface{}{"visibility": "public"}},
					map[string]interface{}{"term": map[string]interface{}{"encoding_status": "ready"}},
				},
			},
		},
		"from": from,
		"size": size,
	}
	if withHighlight {
		body["highlight"] = map[string]interface{}{
			"fields": map[string]interface{}{
				"title":       map[string]interface{}{},
				"description": map[string]interface{}{},
			},
			"pre_tags":  []string{"<mark>"},
			"post_tags": []string{"</mark>"},
		}
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return nil, err
	}

	res, err := esapi.SearchRequest{
		Index: []string{c.index},
		Body:  bytes.NewReader(buf.Bytes()),
	}.Do(ctx, c.es)
	if err != nil {
		slog.ErrorContext(ctx, "elasticsearch_search_request_failed", "index", c.index, "error", err)
		return nil, fmt.Errorf("esclient: search request: %w", err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.IsError() {
		slog.ErrorContext(ctx, "elasticsearch_search_error", "index", c.index, "status", res.StatusCode, "body", string(raw))
		return nil, fmt.Errorf("esclient: search %s: %s", res.Status(), string(raw))
	}

	return parseSearchPublishedResponse(raw, from, size)
}

func parseSearchPublishedResponse(raw []byte, from, size int) (*SearchPublishedResult, error) {
	var wrap struct {
		Hits struct {
			Total struct {
				Value    int64  `json:"value"`
				Relation string `json:"relation"`
			} `json:"total"`
			Hits []struct {
				ID        string              `json:"_id"`
				Score     json.RawMessage     `json:"_score"`
				Source    struct{ VideoID string `json:"video_id"` } `json:"_source"`
				Highlight map[string][]string `json:"highlight"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return nil, fmt.Errorf("esclient: decode search response: %w", err)
	}
	out := &SearchPublishedResult{
		Total: wrap.Hits.Total.Value,
		From:  from,
		Size:  size,
		Hits:  make([]SearchPublishedHit, 0, len(wrap.Hits.Hits)),
	}
	for _, h := range wrap.Hits.Hits {
		vid := h.Source.VideoID
		if vid == "" {
			vid = h.ID
		}
		hit := SearchPublishedHit{
			VideoID:    vid,
			Highlights: h.Highlight,
		}
		if len(h.Score) > 0 && string(h.Score) != "null" {
			var sc float64
			if err := json.Unmarshal(h.Score, &sc); err == nil {
				hit.Score = &sc
			}
		}
		out.Hits = append(out.Hits, hit)
	}
	return out, nil
}

// ParseSearchPagination parses from/size query parameters with defaults and caps.
func ParseSearchPagination(fromStr, sizeStr string) (from, size int) {
	from = 0
	size = defaultSearchSize
	if fromStr != "" {
		if n, err := strconv.Atoi(fromStr); err == nil && n >= 0 {
			from = n
		}
	}
	if sizeStr != "" {
		if n, err := strconv.Atoi(sizeStr); err == nil && n > 0 {
			size = n
		}
	}
	if size > maxSearchSize {
		size = maxSearchSize
	}
	return from, size
}
