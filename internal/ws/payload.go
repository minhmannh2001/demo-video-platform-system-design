package ws

import (
	"video-platform/internal/models"
	"video-platform/internal/streamutil"
)

// EnvelopeVideoUpdatedFromStatus builds a video.updated JSON body aligned with GET .../watch.
func EnvelopeVideoUpdatedFromStatus(publicBaseURL, videoID, status string) ([]byte, error) {
	p := VideoUpdatedPayload{VideoID: videoID, Status: status}
	switch status {
	case models.StatusReady:
		p.ManifestURL = streamutil.ManifestURL(publicBaseURL, videoID)
	case models.StatusProcessing:
		p.Message = "encoding in progress"
	case models.StatusFailed:
		p.Message = "encoding failed"
	}
	return ServerEnvelopeVideoUpdated(p)
}
