package videometaqueue

import "time"

const SchemaV1 = 1

// Ops signal how search index / consumers should treat the video_id.
const (
	OpCreated  = "created"
	OpUpdated  = "updated"
	OpDeleted  = "deleted"
)

// Event is the JSON body on the metadata SQS queue (versioned contract).
type Event struct {
	SchemaVersion      int    `json:"schema_version"`
	VideoID            string `json:"video_id"`
	Op                 string `json:"op"`
	OccurredAt         string `json:"occurred_at"`          // RFC3339Nano
	CorrelationVersion int64  `json:"correlation_version"` // e.g. updated_at unix nano for ordering / idempotency hints
}

// NewEvent builds an event; if versionTime is zero, time.Now().UTC() is used.
func NewEvent(videoID, op string, versionTime time.Time) Event {
	if versionTime.IsZero() {
		versionTime = time.Now().UTC()
	}
	versionTime = versionTime.UTC()
	return Event{
		SchemaVersion:      SchemaV1,
		VideoID:            videoID,
		Op:                 op,
		OccurredAt:         versionTime.Format(time.RFC3339Nano),
		CorrelationVersion: versionTime.UnixNano(),
	}
}
