package esclient

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/elastic/go-elasticsearch/v8/esapi"
)

//go:embed videos-index-template.json
var defaultVideosIndexTemplate []byte

// EnsureVideosIndexSetup applies the embedded composable template and creates the client index if missing.
func (c *Client) EnsureVideosIndexSetup(ctx context.Context) error {
	return c.ensureVideosIndexSetup(ctx, defaultVideosIndexTemplate)
}

func (c *Client) ensureVideosIndexSetup(ctx context.Context, templateJSON []byte) error {
	tplRes, err := esapi.IndicesPutIndexTemplateRequest{
		Name: "videos-template",
		Body: bytes.NewReader(templateJSON),
	}.Do(ctx, c.es)
	if err != nil {
		return fmt.Errorf("esclient: put index template: %w", err)
	}
	defer tplRes.Body.Close()
	if tplRes.IsError() {
		b, _ := io.ReadAll(tplRes.Body)
		return fmt.Errorf("esclient: put index template %s: %s", tplRes.Status(), string(b))
	}

	existsRes, err := esapi.IndicesExistsRequest{Index: []string{c.index}}.Do(ctx, c.es)
	if err != nil {
		return fmt.Errorf("esclient: index exists: %w", err)
	}
	defer existsRes.Body.Close()
	if existsRes.StatusCode == http.StatusOK {
		return nil
	}

	createRes, err := esapi.IndicesCreateRequest{Index: c.index}.Do(ctx, c.es)
	if err != nil {
		return fmt.Errorf("esclient: create index: %w", err)
	}
	defer createRes.Body.Close()
	if createRes.IsError() {
		b, _ := io.ReadAll(createRes.Body)
		if createRes.StatusCode == http.StatusBadRequest && strings.Contains(string(b), "resource_already_exists_exception") {
			return nil
		}
		return fmt.Errorf("esclient: create index %s: %s", createRes.Status(), string(b))
	}
	return nil
}
