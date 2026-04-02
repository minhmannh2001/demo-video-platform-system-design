package models

import "time"

const (
	StatusProcessing = "processing"
	StatusReady      = "ready"
	StatusFailed     = "failed"
)

// Visibility controls catalog / search exposure (separate from encoding Status).
const (
	VisibilityPublic   = "public"
	VisibilityUnlisted = "unlisted"
	VisibilityPrivate  = "private"
)

type Video struct {
	ID            string      `bson:"_id" json:"id"`
	Title         string      `bson:"title" json:"title"`
	Description   string      `bson:"description" json:"description"`
	Uploader      string      `bson:"uploader" json:"uploader"`
	Visibility    string      `bson:"visibility,omitempty" json:"visibility,omitempty"`
	RawS3Key      string      `bson:"raw_s3_key" json:"raw_s3_key"`
	EncodedPrefix string      `bson:"encoded_prefix,omitempty" json:"encoded_prefix,omitempty"`
	ThumbnailKey  string      `bson:"thumbnail_key,omitempty" json:"thumbnail_key,omitempty"` // relative path under stream URL, e.g. thumbnail.jpg
	Renditions    []Rendition `bson:"renditions,omitempty" json:"renditions,omitempty"`
	Status        string      `bson:"status" json:"status"`
	DurationSec   int         `bson:"duration_sec,omitempty" json:"duration_sec,omitempty"`
	CreatedAt     time.Time   `bson:"created_at" json:"created_at"`
	UpdatedAt     time.Time   `bson:"updated_at" json:"updated_at"`

	// API-only (not persisted): filled by handlers for GET /videos and GET /videos/{id} when status=ready.
	ThumbnailURL       string                   `bson:"-" json:"thumbnail_url,omitempty"`
	Qualities          []string                 `bson:"-" json:"qualities,omitempty"`
	PlaybackRenditions []WatchPlaybackRendition `bson:"-" json:"playback_renditions,omitempty"`
}

// Rendition describes one encoded quality profile produced from a source video.
type Rendition struct {
	Quality string `bson:"quality" json:"quality"` // e.g. "360p", "720p"
	Width   int    `bson:"width,omitempty" json:"width,omitempty"`
	Height  int    `bson:"height,omitempty" json:"height,omitempty"`
	Bitrate int    `bson:"bitrate,omitempty" json:"bitrate,omitempty"` // average video bitrate (bps)
	Key     string `bson:"key" json:"key"`                             // relative playlist path under /stream/{videoID}/
}

// EffectiveVisibility returns VisibilityPublic when the document predates the visibility field.
func (v *Video) EffectiveVisibility() string {
	if v == nil || v.Visibility == "" {
		return VisibilityPublic
	}
	return v.Visibility
}

// ValidVisibility reports whether s is a known visibility value.
func ValidVisibility(s string) bool {
	switch s {
	case VisibilityPublic, VisibilityUnlisted, VisibilityPrivate:
		return true
	default:
		return false
	}
}

// WatchPlaybackRendition is a ready-state variant with an absolute playlist URL for the client.
type WatchPlaybackRendition struct {
	Quality     string `json:"quality"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
	Bitrate     int    `json:"bitrate,omitempty"`
	PlaylistURL string `json:"playlist_url"`
}

type WatchResponse struct {
	VideoID      string                   `json:"video_id"`
	Status       string                   `json:"status"`
	ManifestURL  string                   `json:"manifest_url,omitempty"`
	ThumbnailURL string                   `json:"thumbnail_url,omitempty"`
	Qualities    []string                 `json:"qualities,omitempty"`  // e.g. ["360p","720p","auto"]
	Renditions   []WatchPlaybackRendition `json:"renditions,omitempty"` // detail + playlist_url per quality
	Message      string                   `json:"message,omitempty"`
}
