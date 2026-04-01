//go:build integration

package esclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/elasticsearch"

	"video-platform/demo/internal/models"
	"video-platform/demo/internal/search"
)

// TestIntegration_UpsertDeleteRoundTrip requires Docker (Testcontainers) unless ELASTICSEARCH_INTEGRATION_URL is set.
// Run: go test -tags=integration ./internal/search/esclient/ -v -count=1
func TestIntegration_UpsertDeleteRoundTrip(t *testing.T) {
	ctx := context.Background()

	var baseURL string
	var user, pass string

	if u := strings.TrimSpace(os.Getenv("ELASTICSEARCH_INTEGRATION_URL")); u != "" {
		baseURL = u
		user = os.Getenv("ELASTICSEARCH_INTEGRATION_USERNAME")
		pass = os.Getenv("ELASTICSEARCH_INTEGRATION_PASSWORD")
		t.Logf("using ELASTICSEARCH_INTEGRATION_URL=%s", baseURL)
	} else {
		esC, err := elasticsearch.Run(ctx,
			"docker.elastic.co/elasticsearch/elasticsearch:8.15.3",
			testcontainers.WithEnv(map[string]string{
				"xpack.security.enabled":          "false",
				"xpack.security.http.ssl.enabled": "false",
				"ES_JAVA_OPTS":                    "-Xms512m -Xmx512m",
			}),
		)
		if err != nil {
			t.Fatalf("start elasticsearch: %v", err)
		}
		t.Cleanup(func() {
			_ = testcontainers.TerminateContainer(esC)
		})
		baseURL = esC.Settings.Address
		user = esC.Settings.Username
		pass = esC.Settings.Password
	}

	templatePath := filepath.Join("..", "..", "..", "ops", "elasticsearch", "videos-index-template.json")
	templateBody, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("read template: %v", err)
	}

	lowLevel, err := New(Config{
		Addresses:  []string{baseURL},
		Username:   user,
		Password:   pass,
		Index:      "_bootstrap_",
		MaxRetries: 5,
	})
	if err != nil {
		t.Fatal(err)
	}

	tplReq := esapi.IndicesPutIndexTemplateRequest{
		Name: "videos-template-itest",
		Body: bytes.NewReader(templateBody),
	}
	tplRes, err := tplReq.Do(ctx, lowLevel.es)
	if err != nil {
		t.Fatalf("put index template: %v", err)
	}
	defer tplRes.Body.Close()
	if tplRes.IsError() {
		body, _ := io.ReadAll(tplRes.Body)
		t.Fatalf("put index template: %s body=%s", tplRes.Status(), string(body))
	}

	indexName := fmt.Sprintf("videos-itest-%d", time.Now().UnixNano())
	createReq := esapi.IndicesCreateRequest{Index: indexName}
	createRes, err := createReq.Do(ctx, lowLevel.es)
	if err != nil {
		t.Fatalf("create index: %v", err)
	}
	defer createRes.Body.Close()
	if createRes.IsError() {
		body, _ := io.ReadAll(createRes.Body)
		t.Fatalf("create index: %s body=%s", createRes.Status(), string(body))
	}
	t.Cleanup(func() {
		delRes, err := esapi.IndicesDeleteRequest{Index: []string{indexName}}.Do(context.Background(), lowLevel.es)
		if err == nil && delRes != nil {
			_ = delRes.Body.Close()
		}
	})

	client, err := New(Config{
		Addresses:  []string{baseURL},
		Username:   user,
		Password:   pass,
		Index:      indexName,
		MaxRetries: 5,
	})
	if err != nil {
		t.Fatal(err)
	}

	v := &models.Video{
		ID:          "507f1f77bcf86cd799439011",
		Title:       "integration pasta",
		Description: "tomato and basil",
		Uploader:    "itest-uploader",
		Visibility:  models.VisibilityPublic,
		Status:      models.StatusReady,
		CreatedAt:   time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 1, 12, 30, 0, 0, time.UTC),
	}
	doc, err := search.VideoSearchDocFromVideo(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.UpsertVideo(ctx, doc); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	src, ok, err := client.GetVideoSource(ctx, doc.VideoID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected document after upsert")
	}
	var got search.VideoSearchDoc
	if err := json.Unmarshal(src, &got); err != nil {
		t.Fatal(err)
	}
	if got.Title != doc.Title || got.OwnerID != doc.OwnerID || got.EncodingStatus != doc.EncodingStatus {
		t.Fatalf("round-trip mismatch: %+v vs %+v", got, doc)
	}

	if err := client.DeleteVideo(ctx, doc.VideoID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	src2, ok2, err := client.GetVideoSource(ctx, doc.VideoID)
	if err != nil {
		t.Fatal(err)
	}
	if ok2 || src2 != nil {
		t.Fatalf("expected missing document after delete ok=%v src=%s", ok2, string(src2))
	}
	if err := client.DeleteVideo(ctx, doc.VideoID); err != nil {
		t.Fatalf("second delete (idempotent): %v", err)
	}
}
