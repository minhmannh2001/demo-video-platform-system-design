package ws

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Protocol version for all WebSocket JSON envelopes (Phần 2).
const ProtocolV = 1

// Channel values accepted in client "subscribe" / "unsubscribe" (maps to internal topic "catalog").
const ChannelUploads = "uploads"

// TopicCatalog is the internal topic key for the uploads list / catalog invalidate flow.
const TopicCatalog = "catalog"

// Limits per WebSocket connection.
const (
	MaxSubscriptionsPerConn = 32
	// SubscribeRateWindow: at most SubscribeRateMaxSubscribe **new** topic subscribes in this rolling window.
	// Kept ≥ MaxSubscriptionsPerConn so a single connection can reach the topic cap without tripping rate first.
	SubscribeRateWindow       = 10 * time.Second
	SubscribeRateMaxSubscribe = 40
)

// Client message types (JSON field "type").
const (
	TypeSubscribe   = "subscribe"
	TypeUnsubscribe = "unsubscribe"
	TypePing        = "ping"
)

// Server message types.
const (
	TypeHello             = "hello"
	TypeSubscribed        = "subscribed"
	TypeUnsubscribed      = "unsubscribed"
	TypeError             = "error"
	TypePong              = "pong"
	TypeVideoUpdated      = "video.updated"
	TypeCatalogInvalidate = "catalog.invalidate"
)

// Error codes (JSON field "code" on type "error").
const (
	ErrInvalidMessage     = "invalid_message"
	ErrInvalidSubscribe   = "invalid_subscribe"
	ErrRateLimited        = "rate_limited"
	ErrSubscriptionLimit  = "subscription_limit"
	ErrUnknownType        = "unknown_type"
	ErrUnsupportedVersion = "unsupported_version"
	ErrUnknownChannel     = "unknown_channel"
)

// ClientMessage is the JSON body from the browser (text frames).
// Subscribe: exactly one of VideoID (non-empty) or Channel == "uploads".
type ClientMessage struct {
	V       int    `json:"v"`
	Type    string `json:"type"`
	VideoID string `json:"video_id,omitempty"`
	Channel string `json:"channel,omitempty"`
}

// VideoUpdatedPayload aligns with REST watch-style fields for UI updates (subset of models.WatchResponse).
type VideoUpdatedPayload struct {
	VideoID     string `json:"video_id"`
	Status      string `json:"status,omitempty"`
	ManifestURL string `json:"manifest_url,omitempty"`
	Message     string `json:"message,omitempty"`
}

// ParseClientText parses and validates a single client text frame.
func ParseClientText(data []byte) (ClientMessage, error) {
	var m ClientMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return m, err
	}
	if m.V == 0 {
		m.V = 1
	}
	if m.V != ProtocolV {
		return m, fmt.Errorf("%w: v=%d", errUnsupportedVersion, m.V)
	}
	m.Type = strings.TrimSpace(strings.ToLower(m.Type))
	return m, nil
}

var (
	errUnsupportedVersion = errors.New("ws: unsupported protocol version")
	errUnknownChannel     = errors.New("ws: unknown channel")
)

// TopicFromSubscribe returns the internal topic key or an error if the combination is invalid.
func TopicFromSubscribe(videoID, channel string) (string, error) {
	videoID = strings.TrimSpace(videoID)
	ch := strings.TrimSpace(strings.ToLower(channel))
	hasVid := videoID != ""
	hasCh := ch != ""

	switch {
	case hasVid && hasCh:
		return "", errors.New("ws: specify only one of video_id or channel")
	case !hasVid && !hasCh:
		return "", errors.New("ws: need video_id or channel")
	case hasCh && ch != ChannelUploads:
		return "", fmt.Errorf("%w: %q", errUnknownChannel, ch)
	case hasVid:
		return "video:" + videoID, nil
	default:
		return TopicCatalog, nil
	}
}

// IsUnsupportedVersion reports whether err is unknown v.
func IsUnsupportedVersion(err error) bool {
	return errors.Is(err, errUnsupportedVersion)
}

// IsUnknownChannel reports whether err is from an invalid subscribe channel value.
func IsUnknownChannel(err error) bool {
	return errors.Is(err, errUnknownChannel)
}

// ServerEnvelopeVideoUpdated is the JSON body for server → client push on `video:{id}` (Phần 3+).
func ServerEnvelopeVideoUpdated(p VideoUpdatedPayload) ([]byte, error) {
	return json.Marshal(struct {
		Type    string              `json:"type"`
		V       int                 `json:"v"`
		Payload VideoUpdatedPayload `json:"payload"`
	}{
		Type:    TypeVideoUpdated,
		V:       ProtocolV,
		Payload: p,
	})
}

// ServerEnvelopeCatalogInvalidate is the JSON body for catalog refresh signal (Phần 3+).
func ServerEnvelopeCatalogInvalidate() ([]byte, error) {
	return json.Marshal(map[string]any{
		"type": TypeCatalogInvalidate,
		"v":    ProtocolV,
	})
}
