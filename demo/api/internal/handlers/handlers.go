package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	"video-platform/demo/internal/config"
	"video-platform/demo/internal/models"
	"video-platform/demo/internal/search/esclient"
	"video-platform/demo/internal/store"
	"video-platform/demo/internal/streamutil"
	"video-platform/demo/internal/tracing"
	"video-platform/demo/internal/videometaqueue"
	"video-platform/demo/internal/ws"
	"video-platform/demo/internal/wsevents"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.opentelemetry.io/otel/attribute"
)

type Handler struct {
	cfg               config.Config
	s3                S3API
	sqs               SQSAPI
	queueURL          string
	metadataQueueURL  string
	metadataPublisher videometaqueue.Publisher
	videos            VideoRepository
	videoCache        VideoCacher
	videoSearch       VideoSearch
	wsBridge          *wsevents.Bridge
}

// New wires HTTP handlers with concrete AWS/Mongo/Redis clients from main.
// metadataPublisher may be nil; it is treated as videometaqueue.Noop.
// videoSearch may be nil; GET /videos/search returns 503 when unset.
// wsBridge may be nil; realtime WebSocket push is disabled.
func New(cfg config.Config, s3cli S3API, sqscli SQSAPI, encodeQueueURL, metadataQueueURL string, videos VideoRepository, vc VideoCacher, metadataPublisher videometaqueue.Publisher, videoSearch VideoSearch, wsBridge *wsevents.Bridge) *Handler {
	if metadataPublisher == nil {
		metadataPublisher = videometaqueue.Noop{}
	}
	return &Handler{
		cfg:               cfg,
		s3:                s3cli,
		sqs:               sqscli,
		queueURL:          encodeQueueURL,
		metadataQueueURL:  metadataQueueURL,
		metadataPublisher: metadataPublisher,
		videos:            videos,
		videoCache:        vc,
		videoSearch:       videoSearch,
		wsBridge:          wsBridge,
	}
}

func (h *Handler) pushVideoWS(ctx context.Context, videoID, status string) {
	if h.wsBridge == nil {
		return
	}
	c, sp := tracing.Start(ctx, "ws.video.publish",
		attribute.String("ws.event_type", ws.TypeVideoUpdated),
		attribute.String("video.id", videoID),
		attribute.String("video.status", status),
	)
	body, err := ws.EnvelopeVideoUpdatedFromStatus(h.cfg.PublicBaseURL, videoID, status)
	if err != nil {
		tracing.Finish(sp, err)
		slog.WarnContext(ctx, "ws_video_envelope_failed", "video_id", videoID, "status", status, "error", err)
		return
	}
	topic := "video:" + videoID
	err = h.wsBridge.Publish(c, topic, body)
	tracing.Finish(sp, err)
	if err != nil {
		slog.WarnContext(ctx, "ws_video_publish_failed", "video_id", videoID, "status", status, "topic", topic, "error", err)
		return
	}
	slog.DebugContext(ctx, "ws_video_published", "video_id", videoID, "status", status, "topic", topic)
}

func (h *Handler) pushCatalogInvalidate(ctx context.Context) {
	if h.wsBridge == nil {
		return
	}
	c, sp := tracing.Start(ctx, "ws.catalog.publish",
		attribute.String("ws.event_type", ws.TypeCatalogInvalidate),
		attribute.String("ws.topic", ws.TopicCatalog),
	)
	body, err := ws.ServerEnvelopeCatalogInvalidate()
	if err != nil {
		tracing.Finish(sp, err)
		slog.WarnContext(ctx, "ws_catalog_envelope_failed", "error", err)
		return
	}
	err = h.wsBridge.Publish(c, ws.TopicCatalog, body)
	tracing.Finish(sp, err)
	if err != nil {
		slog.WarnContext(ctx, "ws_catalog_publish_failed", "topic", ws.TopicCatalog, "error", err)
		return
	}
	slog.DebugContext(ctx, "ws_catalog_published", "topic", ws.TopicCatalog)
}

func (h *Handler) publishVideoMetadata(ctx context.Context, ev videometaqueue.Event) {
	c, sp := tracing.Start(ctx, "sqs.SendMessage",
		attribute.String("messaging.system", "aws_sqs"),
		attribute.String("messaging.destination.name", h.metadataQueueURL),
		attribute.String("videometa.op", ev.Op),
		attribute.String("video.id", ev.VideoID),
	)
	err := h.metadataPublisher.Publish(c, ev)
	tracing.Finish(sp, err)
	if err != nil {
		slog.ErrorContext(ctx, "metadata_index_enqueue_failed",
			"error", err,
			"video_id", ev.VideoID,
			"op", ev.Op,
		)
		return
	}
	slog.InfoContext(ctx, "metadata_index_enqueued",
		"video_id", ev.VideoID,
		"op", ev.Op,
		"schema_version", ev.SchemaVersion,
		"correlation_version", ev.CorrelationVersion,
	)
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/videos/upload", h.Upload)
	r.Get("/videos/search", h.SearchVideos)
	r.Get("/videos", h.ListVideos)
	r.Get("/videos/{id}", h.GetVideo)
	r.Patch("/videos/{id}", h.PatchVideo)
	r.Delete("/videos/{id}", h.DeleteVideo)
	r.Get("/videos/{id}/watch", h.Watch)
	r.Handle("/stream/*", http.StripPrefix("/stream/", http.HandlerFunc(h.StreamObject)))
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return r
}

// SearchVideos handles GET /videos/search?q=... (Elasticsearch only; no Mongo).
func (h *Handler) SearchVideos(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.videoSearch == nil {
		http.Error(w, "search unavailable", http.StatusServiceUnavailable)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		http.Error(w, "q query parameter required", http.StatusBadRequest)
		return
	}
	from, size := esclient.ParseSearchPagination(r.URL.Query().Get("from"), r.URL.Query().Get("size"))
	highlight := strings.EqualFold(r.URL.Query().Get("highlight"), "true") ||
		r.URL.Query().Get("highlight") == "1"

	ctx, sp := tracing.Start(ctx, "video.search",
		attribute.Int("search.pagination.from", from),
		attribute.Int("search.pagination.size", size),
		attribute.Bool("search.highlight", highlight),
		attribute.Int("search.query_length", len(q)),
		attribute.Bool("search.redis_cache_enabled", h.cfg.SearchCacheTTLSec > 0),
	)
	result, err := h.videoSearch.SearchPublishedVideos(ctx, q, from, size, highlight)
	tracing.Finish(sp, err)
	if err != nil {
		slog.ErrorContext(ctx, "video_search_failed", "error", err, "q", q)
		http.Error(w, "search failed", http.StatusBadGateway)
		return
	}
	// When Redis search cache is on, [cache.CachedPublishedSearch] logs hit/miss; avoid duplicate lines.
	if h.cfg.SearchCacheTTLSec <= 0 {
		slog.InfoContext(ctx, "video_search",
			"q", q,
			"total", result.Total,
			"returned", len(result.Hits),
			"from", result.From,
			"size", result.Size,
		)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		http.Error(w, "bad multipart form", http.StatusBadRequest)
		return
	}
	title := r.FormValue("title")
	if title == "" {
		http.Error(w, "title required", http.StatusBadRequest)
		return
	}
	desc := r.FormValue("description")
	uploader := r.FormValue("uploader")
	file, hdr, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	id := primitive.NewObjectID().Hex()
	ext := path.Ext(hdr.Filename)
	if ext == "" {
		ext = ".bin"
	}
	rawKey, err := streamutil.RawUploadObjectKey(id, ext)
	if err != nil {
		http.Error(w, "internal key error", http.StatusInternalServerError)
		return
	}

	slog.InfoContext(ctx, "upload_started",
		"video_id", id,
		"title", title,
		"uploader", uploader,
		"raw_s3_key", rawKey,
	)

	{
		c, sp := tracing.Start(ctx, "s3.PutObject",
			attribute.String("aws.service", "S3"),
			attribute.String("s3.bucket", h.cfg.S3RawBucket),
			attribute.String("s3.key", rawKey),
		)
		_, err = h.s3.PutObject(c, &s3.PutObjectInput{
			Bucket: aws.String(h.cfg.S3RawBucket),
			Key:    aws.String(rawKey),
			Body:   file,
		})
		tracing.Finish(sp, err)
		if err != nil {
			http.Error(w, "s3 upload failed", http.StatusInternalServerError)
			return
		}
	}
	slog.InfoContext(ctx, "upload_raw_complete", "video_id", id, "raw_s3_key", rawKey)

	v := &models.Video{
		ID:          id,
		Title:       title,
		Description: desc,
		Uploader:    uploader,
		Visibility:  models.VisibilityPublic,
		RawS3Key:    rawKey,
		Status:      models.StatusProcessing,
	}
	{
		c, sp := tracing.Start(ctx, "mongo.videos.insertOne",
			attribute.String("db.system", "mongodb"),
			attribute.String("db.mongodb.collection", "videos"),
		)
		err = h.videos.Create(c, v)
		tracing.Finish(sp, err)
		if err != nil {
			http.Error(w, "db insert failed", http.StatusInternalServerError)
			return
		}
	}
	slog.InfoContext(ctx, "video_record_created", "video_id", id, "status", v.Status)

	h.publishVideoMetadata(ctx, videometaqueue.NewEvent(id, videometaqueue.OpCreated, v.UpdatedAt))
	h.pushVideoWS(ctx, id, models.StatusProcessing)
	h.pushCatalogInvalidate(ctx)

	body, _ := json.Marshal(map[string]string{"video_id": id})
	{
		c, sp := tracing.Start(ctx, "sqs.SendMessage",
			attribute.String("messaging.system", "aws_sqs"),
			attribute.String("messaging.destination.name", h.queueURL),
		)
		sendOut, err := h.sqs.SendMessage(c, &sqs.SendMessageInput{
			QueueUrl:          aws.String(h.queueURL),
			MessageBody:       aws.String(string(body)),
			MessageAttributes: tracing.InjectIntoSQSAttributes(c),
		})
		tracing.Finish(sp, err)
		if err != nil {
			http.Error(w, "enqueue failed", http.StatusInternalServerError)
			return
		}
		slog.InfoContext(ctx, "encode_job_enqueued",
			"video_id", id,
			"sqs_message_id", aws.ToString(sendOut.MessageId),
		)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"id": id, "status": models.StatusProcessing})
}

func (h *Handler) ListVideos(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx, span := tracing.Start(ctx, "mongo.videos.find",
		attribute.String("db.system", "mongodb"),
		attribute.String("db.mongodb.collection", "videos"),
	)
	var err error
	defer func() { tracing.Finish(span, err) }()

	list, err := h.videos.List(ctx, 50)
	if err != nil {
		http.Error(w, "list failed", http.StatusInternalServerError)
		return
	}
	slog.InfoContext(ctx, "list_videos", "count", len(list))
	for i := range list {
		enrichPlaybackFields(&list[i], h.cfg.PublicBaseURL)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

func (h *Handler) GetVideo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	if h.videoCache != nil {
		c, sp := tracing.Start(ctx, "redis.video.get",
			attribute.String("db.system", "redis"),
			attribute.String("db.operation", "GET"),
			attribute.String("redis.key", "video:"+id),
		)
		cv, getErr := h.videoCache.Get(c, id)
		switch {
		case getErr == nil && cv != nil:
			tracing.Finish(sp, nil)
			slog.InfoContext(ctx, "get_video", "video_id", id, "cache_hit", true)
			enrichPlaybackFields(cv, h.cfg.PublicBaseURL)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(cv)
			return
		case errors.Is(getErr, redis.Nil):
			tracing.Finish(sp, nil)
		default:
			tracing.Finish(sp, getErr)
		}
	}

	ctx, span := tracing.Start(ctx, "mongo.videos.findOne",
		attribute.String("db.system", "mongodb"),
		attribute.String("db.mongodb.collection", "videos"),
	)
	var err error
	defer func() { tracing.Finish(span, err) }()

	v, err := h.videos.GetByID(ctx, id)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if v == nil {
		slog.WarnContext(ctx, "get_video_not_found", "video_id", id)
		http.NotFound(w, r)
		return
	}
	if h.videoCache != nil {
		c, sp := tracing.Start(ctx, "redis.video.set",
			attribute.String("db.system", "redis"),
			attribute.String("db.operation", "SET"),
			attribute.String("redis.key", "video:"+v.ID),
		)
		setErr := h.videoCache.Set(c, v)
		tracing.Finish(sp, setErr)
	}
	slog.InfoContext(ctx, "get_video", "video_id", id, "cache_hit", false, "status", v.Status)
	enrichPlaybackFields(v, h.cfg.PublicBaseURL)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (h *Handler) PatchVideo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	var body struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Visibility  string `json:"visibility"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.Title) == "" {
		http.Error(w, "title required", http.StatusBadRequest)
		return
	}
	{
		c, sp := tracing.Start(ctx, "mongo.videos.updateMetadata",
			attribute.String("db.system", "mongodb"),
			attribute.String("db.mongodb.collection", "videos"),
		)
		err := h.videos.UpdateMetadata(c, id, body.Title, body.Description, body.Visibility)
		tracing.Finish(sp, err)
		if err != nil {
			if errors.Is(err, store.ErrVideoNotFound) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "update failed", http.StatusInternalServerError)
			return
		}
	}
	v, err := h.videos.GetByID(ctx, id)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if v == nil {
		http.NotFound(w, r)
		return
	}
	if h.videoCache != nil {
		c, sp := tracing.Start(ctx, "redis.video.del",
			attribute.String("db.system", "redis"),
			attribute.String("db.operation", "DEL"),
			attribute.String("redis.key", "video:"+id),
		)
		delErr := h.videoCache.Del(c, id)
		tracing.Finish(sp, delErr)
	}
	h.publishVideoMetadata(ctx, videometaqueue.NewEvent(id, videometaqueue.OpUpdated, v.UpdatedAt))
	h.pushCatalogInvalidate(ctx)
	slog.InfoContext(ctx, "video_metadata_patched", "video_id", id)
	enrichPlaybackFields(v, h.cfg.PublicBaseURL)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (h *Handler) Watch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	ctx, span := tracing.Start(ctx, "mongo.videos.findOne",
		attribute.String("db.system", "mongodb"),
		attribute.String("db.mongodb.collection", "videos"),
	)
	var err error
	defer func() { tracing.Finish(span, err) }()

	v, err := h.videos.GetByID(ctx, id)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if v == nil {
		slog.WarnContext(ctx, "watch_video_not_found", "video_id", id)
		http.NotFound(w, r)
		return
	}
	slog.InfoContext(ctx, "watch_status", "video_id", id, "video_status", v.Status)
	resp := watchResponseFromVideo(v, h.cfg.PublicBaseURL)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handler) DeleteVideo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	var v *models.Video
	{
		c, sp := tracing.Start(ctx, "mongo.videos.findOne",
			attribute.String("db.system", "mongodb"),
			attribute.String("db.mongodb.collection", "videos"),
		)
		var getErr error
		v, getErr = h.videos.GetByID(c, id)
		tracing.Finish(sp, getErr)
		if getErr != nil {
			http.Error(w, "invalid video id", http.StatusBadRequest)
			return
		}
		if v == nil {
			slog.WarnContext(ctx, "delete_video_not_found", "video_id", id)
			http.NotFound(w, r)
			return
		}
	}

	slog.InfoContext(ctx, "delete_video_started", "video_id", id)

	if v.RawS3Key != "" {
		c, sp := tracing.Start(ctx, "s3.DeleteObject",
			attribute.String("aws.service", "S3"),
			attribute.String("s3.bucket", h.cfg.S3RawBucket),
			attribute.String("s3.key", v.RawS3Key),
		)
		_, rawDelErr := h.s3.DeleteObject(c, &s3.DeleteObjectInput{
			Bucket: aws.String(h.cfg.S3RawBucket),
			Key:    aws.String(v.RawS3Key),
		})
		tracing.Finish(sp, rawDelErr)
	}
	if v.EncodedPrefix != "" {
		cEnc, spEnc := tracing.Start(ctx, "s3.encoded.delete_prefix",
			attribute.String("aws.service", "S3"),
			attribute.String("s3.bucket", h.cfg.S3EncodedBucket),
			attribute.String("s3.prefix", v.EncodedPrefix),
		)
		var token *string
		for {
			out, listErr := h.s3.ListObjectsV2(cEnc, &s3.ListObjectsV2Input{
				Bucket:            aws.String(h.cfg.S3EncodedBucket),
				Prefix:            aws.String(v.EncodedPrefix),
				ContinuationToken: token,
			})
			if listErr != nil {
				tracing.Finish(spEnc, listErr)
				http.Error(w, "storage cleanup failed", http.StatusInternalServerError)
				return
			}
			for _, obj := range out.Contents {
				if obj.Key == nil {
					continue
				}
				_, _ = h.s3.DeleteObject(cEnc, &s3.DeleteObjectInput{
					Bucket: aws.String(h.cfg.S3EncodedBucket),
					Key:    obj.Key,
				})
			}
			if !aws.ToBool(out.IsTruncated) || out.NextContinuationToken == nil {
				break
			}
			token = out.NextContinuationToken
		}
		tracing.Finish(spEnc, nil)
	}

	if h.videoCache != nil {
		c, sp := tracing.Start(ctx, "redis.video.del",
			attribute.String("db.system", "redis"),
			attribute.String("db.operation", "DEL"),
			attribute.String("redis.key", "video:"+id),
		)
		delCacheErr := h.videoCache.Del(c, id)
		tracing.Finish(sp, delCacheErr)
	}

	{
		c, sp := tracing.Start(ctx, "mongo.videos.deleteOne",
			attribute.String("db.system", "mongodb"),
			attribute.String("db.mongodb.collection", "videos"),
		)
		deleted, delErr := h.videos.DeleteByID(c, id)
		tracing.Finish(sp, delErr)
		if delErr != nil {
			http.Error(w, "db delete failed", http.StatusInternalServerError)
			return
		}
		if !deleted {
			http.NotFound(w, r)
			return
		}
	}
	h.publishVideoMetadata(ctx, videometaqueue.NewEvent(id, videometaqueue.OpDeleted, time.Now().UTC()))
	h.pushCatalogInvalidate(ctx)
	slog.InfoContext(ctx, "delete_video_complete", "video_id", id)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) StreamObject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	p := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.SplitN(p, "/", 2)
	if len(parts) < 2 {
		http.NotFound(w, r)
		return
	}
	videoID, file := parts[0], parts[1]
	key, keyErr := streamutil.EncodedHLSObjectKey(videoID, file)
	if keyErr != nil {
		switch {
		case errors.Is(keyErr, streamutil.ErrInvalidVideoID):
			slog.WarnContext(ctx, "stream_path_invalid", "video_id", videoID, "reason", "invalid_video_id")
			http.NotFound(w, r)
		case errors.Is(keyErr, streamutil.ErrInvalidRelativeKey):
			slog.WarnContext(ctx, "stream_path_invalid", "video_id", videoID, "reason", "invalid_relative_key", "file", file)
			http.Error(w, "invalid path", http.StatusBadRequest)
		default:
			http.NotFound(w, r)
		}
		return
	}

	// Chỉ log manifest (.m3u8) — không log từng segment .ts (rất nhiều request).
	if strings.HasSuffix(strings.ToLower(file), ".m3u8") {
		slog.InfoContext(ctx, "stream_manifest", "video_id", videoID, "file", file)
	}

	ctx, span := tracing.Start(ctx, "s3.GetObject",
		attribute.String("aws.service", "S3"),
		attribute.String("s3.bucket", h.cfg.S3EncodedBucket),
		attribute.String("s3.key", key),
	)
	var err error
	defer func() { tracing.Finish(span, err) }()

	out, err := h.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(h.cfg.S3EncodedBucket),
		Key:    aws.String(key),
	})
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer out.Body.Close()

	ct := streamutil.ContentTypeByFilename(file)
	if out.ContentType != nil && *out.ContentType != "" {
		ct = *out.ContentType
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, err = io.Copy(w, out.Body)
}

// enrichPlaybackFields sets JSON-only fields for ready videos (URLs for thumbnail and per-rendition playlists).
func enrichPlaybackFields(v *models.Video, publicBase string) {
	if v == nil {
		return
	}
	v.ThumbnailURL = ""
	v.Qualities = nil
	v.PlaybackRenditions = nil
	if publicBase == "" || v.Status != models.StatusReady {
		return
	}
	if v.ThumbnailKey != "" {
		if u, err := streamutil.StreamPublicURL(publicBase, v.ID, v.ThumbnailKey); err == nil {
			v.ThumbnailURL = u
		}
	}
	if v.ThumbnailURL == "" {
		v.ThumbnailURL = streamutil.ThumbnailURL(publicBase, v.ID)
	}
	seen := make(map[string]struct{})
	for _, r := range v.Renditions {
		if r.Quality == "" {
			continue
		}
		if _, ok := seen[r.Quality]; ok {
			continue
		}
		seen[r.Quality] = struct{}{}
		v.Qualities = append(v.Qualities, r.Quality)
	}
	v.Qualities = append(v.Qualities, "auto")
	for _, r := range v.Renditions {
		if r.Key == "" {
			continue
		}
		pu, err := streamutil.RenditionPlaylistURL(publicBase, v.ID, r.Key)
		if err != nil {
			continue
		}
		v.PlaybackRenditions = append(v.PlaybackRenditions, models.WatchPlaybackRendition{
			Quality: r.Quality, Width: r.Width, Height: r.Height, Bitrate: r.Bitrate, PlaylistURL: pu,
		})
	}
}

func watchResponseFromVideo(v *models.Video, publicBase string) models.WatchResponse {
	if v == nil {
		return models.WatchResponse{}
	}
	resp := models.WatchResponse{VideoID: v.ID, Status: v.Status}
	switch v.Status {
	case models.StatusReady:
		resp.ManifestURL = streamutil.ManifestURL(publicBase, v.ID)
		tmp := *v
		enrichPlaybackFields(&tmp, publicBase)
		resp.ThumbnailURL = tmp.ThumbnailURL
		resp.Qualities = tmp.Qualities
		resp.Renditions = tmp.PlaybackRenditions
	case models.StatusProcessing:
		resp.Message = "encoding in progress"
	default:
		resp.Message = "encoding failed"
	}
	return resp
}

// corsAllowHeaders merges a fixed allowlist with the browser's preflight
// Access-Control-Request-Headers so names match exactly (e.g. traceparent for OTel).
func corsAllowHeaders(r *http.Request) string {
	parts := []string{"Content-Type", "Range", "traceparent", "tracestate", "baggage"}
	seen := make(map[string]struct{}, len(parts)+8)
	for _, p := range parts {
		seen[strings.ToLower(strings.TrimSpace(p))] = struct{}{}
	}
	if extra := r.Header.Get("Access-Control-Request-Headers"); extra != "" {
		for _, p := range strings.Split(extra, ",") {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			k := strings.ToLower(p)
			if _, ok := seen[k]; ok {
				continue
			}
			seen[k] = struct{}{}
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, ", ")
}

func CORSMiddleware(origins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool)
	for _, o := range origins {
		allowed[o] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if allowed[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			// hls.js may send Range on segment requests; preflight must allow it.
			w.Header().Set("Access-Control-Allow-Headers", corsAllowHeaders(r))
			// Let clients read partial-content headers on cross-origin segment responses.
			w.Header().Set("Access-Control-Expose-Headers", "Content-Range, Accept-Ranges, Content-Length")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Warmup pings dependencies (optional).
func (h *Handler) Warmup(ctx context.Context) error {
	_, err := h.s3.ListBuckets(ctx, &s3.ListBucketsInput{})
	return err
}
