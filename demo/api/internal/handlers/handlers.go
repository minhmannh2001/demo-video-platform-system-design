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
	"video-platform/demo/internal/streamutil"
	"video-platform/demo/internal/tracing"
	"video-platform/demo/internal/videometaqueue"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Handler struct {
	cfg                config.Config
	s3                 S3API
	sqs                SQSAPI
	queueURL           string
	metadataQueueURL   string
	metadataPublisher  videometaqueue.Publisher
	videos             VideoRepository
	videoCache         VideoCacher
}

// New wires HTTP handlers with concrete AWS/Mongo/Redis clients from main.
// metadataPublisher may be nil; it is treated as videometaqueue.Noop.
func New(cfg config.Config, s3cli S3API, sqscli SQSAPI, encodeQueueURL, metadataQueueURL string, videos VideoRepository, vc VideoCacher, metadataPublisher videometaqueue.Publisher) *Handler {
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
	}
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
	r.Get("/videos", h.ListVideos)
	r.Get("/videos/{id}", h.GetVideo)
	r.Delete("/videos/{id}", h.DeleteVideo)
	r.Get("/videos/{id}/watch", h.Watch)
	r.Handle("/stream/*", http.StripPrefix("/stream/", http.HandlerFunc(h.StreamObject)))
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return r
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
	resp := models.WatchResponse{VideoID: id, Status: v.Status}
	if v.Status == models.StatusReady {
		resp.ManifestURL = streamutil.ManifestURL(h.cfg.PublicBaseURL, id)
	} else if v.Status == models.StatusProcessing {
		resp.Message = "encoding in progress"
	} else {
		resp.Message = "encoding failed"
	}
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
