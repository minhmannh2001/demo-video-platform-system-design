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
	ID            string    `bson:"_id" json:"id"`
	Title         string    `bson:"title" json:"title"`
	Description   string    `bson:"description" json:"description"`
	Uploader      string    `bson:"uploader" json:"uploader"`
	Visibility    string    `bson:"visibility,omitempty" json:"visibility,omitempty"`
	RawS3Key      string    `bson:"raw_s3_key" json:"raw_s3_key"`
	EncodedPrefix string    `bson:"encoded_prefix,omitempty" json:"encoded_prefix,omitempty"`
	Status        string    `bson:"status" json:"status"`
	DurationSec   int       `bson:"duration_sec,omitempty" json:"duration_sec,omitempty"`
	CreatedAt     time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt     time.Time `bson:"updated_at" json:"updated_at"`
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

type WatchResponse struct {
	VideoID     string `json:"video_id"`
	Status      string `json:"status"`
	ManifestURL string `json:"manifest_url,omitempty"`
	Message     string `json:"message,omitempty"`
}
