package esclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	"github.com/elastic/go-elasticsearch/v8/esapi"

	"video-platform/demo/internal/search"
)

func bulkNDJSONBody(index string, docs []*search.VideoSearchDoc) ([]byte, error) {
	var buf bytes.Buffer
	for _, doc := range docs {
		if doc == nil || doc.VideoID == "" {
			continue
		}
		meta, err := json.Marshal(map[string]interface{}{
			"index": map[string]interface{}{
				"_index": index,
				"_id":    doc.VideoID,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("esclient: bulk encode meta: %w", err)
		}
		src, err := json.Marshal(doc)
		if err != nil {
			return nil, fmt.Errorf("esclient: bulk encode doc: %w", err)
		}
		buf.Write(meta)
		buf.WriteByte('\n')
		buf.Write(src)
		buf.WriteByte('\n')
	}
	return buf.Bytes(), nil
}

// BulkUpsertSearchDocs indexes documents with the Elasticsearch _bulk API (index action, document id = video_id).
// refresh: "false" during large backfills; use "wait_for" or "true" on the last batch or call IndicesRefresh separately.
func (c *Client) BulkUpsertSearchDocs(ctx context.Context, docs []*search.VideoSearchDoc, refresh string) error {
	if len(docs) == 0 {
		return nil
	}
	if refresh == "" {
		refresh = "false"
	}
	body, err := bulkNDJSONBody(c.index, docs)
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return nil
	}

	res, err := esapi.BulkRequest{
		Body:    bytes.NewReader(body),
		Refresh: refresh,
	}.Do(ctx, c.es)
	if err != nil {
		return fmt.Errorf("esclient: bulk request: %w", err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.IsError() {
		return fmt.Errorf("esclient: bulk %s: %s", res.Status(), string(raw))
	}

	var bulkResp struct {
		Errors bool `json:"errors"`
		Items  []map[string]struct {
			Status int             `json:"status"`
			Error  json.RawMessage `json:"error"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &bulkResp); err != nil {
		return fmt.Errorf("esclient: bulk decode response: %w", err)
	}
	if !bulkResp.Errors {
		return nil
	}
	var firstErr string
	for _, item := range bulkResp.Items {
		for _, r := range item {
			if r.Status >= 300 && len(r.Error) > 0 {
				firstErr = string(r.Error)
				break
			}
		}
		if firstErr != "" {
			break
		}
	}
	slog.ErrorContext(ctx, "elasticsearch_bulk_item_errors", "index", c.index, "sample_error", firstErr)
	return fmt.Errorf("esclient: bulk completed with errors (see logs; sample=%s)", truncateRunes(firstErr, 200))
}

func truncateRunes(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
