package search

import (
	"errors"
	"time"

	"video-platform/internal/models"
)

var (
	ErrNilVideo     = errors.New("search: nil video")
	ErrEmptyVideoID = errors.New("search: empty video id")
)

// VideoSearchDoc is the JSON body for the Elasticsearch index defined in
// ops/elasticsearch/videos-index-template.json (field names must stay in sync).
// It intentionally excludes storage paths and other non-search fields (e.g. raw_s3_key).
type VideoSearchDoc struct {
	VideoID        string    `json:"video_id"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	OwnerID        string    `json:"owner_id"`
	EncodingStatus string    `json:"encoding_status"`
	Visibility     string    `json:"visibility"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// VideoSearchDocFromVideo maps domain metadata to the Elasticsearch document.
func VideoSearchDocFromVideo(v *models.Video) (*VideoSearchDoc, error) {
	if v == nil {
		return nil, ErrNilVideo
	}
	if v.ID == "" {
		return nil, ErrEmptyVideoID
	}
	return &VideoSearchDoc{
		VideoID:        v.ID,
		Title:          v.Title,
		Description:    v.Description,
		OwnerID:        v.Uploader,
		EncodingStatus: v.Status,
		Visibility:     v.EffectiveVisibility(),
		CreatedAt:      v.CreatedAt,
		UpdatedAt:      v.UpdatedAt,
	}, nil
}
