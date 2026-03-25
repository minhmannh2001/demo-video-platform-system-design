package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path"
	"strings"

	"video-platform/demo/internal/config"
	"video-platform/demo/internal/models"
	"video-platform/demo/internal/streamutil"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Handler struct {
	cfg         config.Config
	s3          S3API
	sqs         SQSAPI
	queueURL    string
	videos      VideoRepository
	videoCache  VideoCacher
}

// New wires HTTP handlers with concrete AWS/Mongo/Redis clients from main.
func New(cfg config.Config, s3cli S3API, sqscli SQSAPI, queueURL string, videos VideoRepository, vc VideoCacher) *Handler {
	return &Handler{
		cfg:        cfg,
		s3:         s3cli,
		sqs:        sqscli,
		queueURL:   queueURL,
		videos:     videos,
		videoCache: vc,
	}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/videos/upload", h.Upload)
	r.Get("/videos", h.ListVideos)
	r.Get("/videos/{id}", h.GetVideo)
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

	_, err = h.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(h.cfg.S3RawBucket),
		Key:    aws.String(rawKey),
		Body:   file,
	})
	if err != nil {
		http.Error(w, "s3 upload failed", http.StatusInternalServerError)
		return
	}

	v := &models.Video{
		ID:          id,
		Title:       title,
		Description: desc,
		Uploader:    uploader,
		RawS3Key:    rawKey,
		Status:      models.StatusProcessing,
	}
	if err := h.videos.Create(ctx, v); err != nil {
		http.Error(w, "db insert failed", http.StatusInternalServerError)
		return
	}

	body, _ := json.Marshal(map[string]string{"video_id": id})
	_, err = h.sqs.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(h.queueURL),
		MessageBody: aws.String(string(body)),
	})
	if err != nil {
		http.Error(w, "enqueue failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"id": id, "status": models.StatusProcessing})
}

func (h *Handler) ListVideos(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	list, err := h.videos.List(ctx, 50)
	if err != nil {
		http.Error(w, "list failed", http.StatusInternalServerError)
		return
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
		if v, err := h.videoCache.Get(ctx, id); err == nil && v != nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(v)
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
		_ = h.videoCache.Set(ctx, v)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (h *Handler) Watch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	v, err := h.videos.GetByID(ctx, id)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if v == nil {
		http.NotFound(w, r)
		return
	}
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
			http.NotFound(w, r)
		case errors.Is(keyErr, streamutil.ErrInvalidRelativeKey):
			http.Error(w, "invalid path", http.StatusBadRequest)
		default:
			http.NotFound(w, r)
		}
		return
	}

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
	_, _ = io.Copy(w, out.Body)
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
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
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
