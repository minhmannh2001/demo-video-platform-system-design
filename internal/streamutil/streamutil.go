package streamutil

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	ErrInvalidVideoID   = errors.New("invalid video id")
	ErrInvalidRelativeKey = errors.New("invalid relative path")
)

// Stream URL layout (public, same origin as API):
//   {PublicBaseURL}/stream/{videoObjectID}/{relativeFile}
// relativeFile is validated like S3 keys under videos/{id}/hls/ (no ".." or leading slash).
// Use StreamPublicURL / ManifestURL / ThumbnailURL so handlers and workers stay consistent.

// ManifestStreamRelative is the default master playlist path under /stream/{id}/.
const ManifestStreamRelative = "master.m3u8"

// ContentTypeByFilename picks a Content-Type for HLS segments by file extension.
func ContentTypeByFilename(filename string) string {
	switch strings.ToLower(path.Ext(filename)) {
	case ".m3u8":
		return "application/vnd.apple.mpegurl"
	case ".ts":
		return "video/mp2t"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

// ThumbnailStreamRelative is the path segment after /stream/{videoID}/ for the poster image.
const ThumbnailStreamRelative = "thumbnail.jpg"

// StreamPublicURL returns the browser URL for an encoded object served by GET /stream/{id}/*.
func StreamPublicURL(publicBaseURL, videoIDHex, relativeFile string) (string, error) {
	if _, err := EncodedHLSObjectKey(videoIDHex, relativeFile); err != nil {
		return "", err
	}
	base := strings.TrimRight(publicBaseURL, "/")
	return fmt.Sprintf("%s/stream/%s/%s", base, videoIDHex, relativeFile), nil
}

// ThumbnailURL builds the public URL for the encoded thumbnail (same host as HLS manifest).
func ThumbnailURL(publicBaseURL, videoIDHex string) string {
	u, err := StreamPublicURL(publicBaseURL, videoIDHex, ThumbnailStreamRelative)
	if err != nil {
		return ""
	}
	return u
}

// EncodedHLSObjectKey returns the S3 object key for an encoded HLS file under videos/{id}/hls/.
func EncodedHLSObjectKey(videoIDHex, relativeFile string) (string, error) {
	if _, err := primitive.ObjectIDFromHex(videoIDHex); err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidVideoID, err)
	}
	if err := validateRelativeHLSPath(relativeFile); err != nil {
		return "", err
	}
	return fmt.Sprintf("videos/%s/hls/%s", videoIDHex, relativeFile), nil
}

func validateRelativeHLSPath(rel string) error {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return ErrInvalidRelativeKey
	}
	if strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, "\\") {
		return ErrInvalidRelativeKey
	}
	if strings.Contains(rel, "..") {
		return ErrInvalidRelativeKey
	}
	return nil
}

// ManifestURL builds the public URL for master.m3u8 (API stream path).
func ManifestURL(publicBaseURL, videoIDHex string) string {
	u, err := StreamPublicURL(publicBaseURL, videoIDHex, ManifestStreamRelative)
	if err != nil {
		return ""
	}
	return u
}

// RenditionPlaylistURL builds the public URL for a variant playlist (relative key from models.Rendition.Key).
func RenditionPlaylistURL(publicBaseURL, videoIDHex, renditionKey string) (string, error) {
	return StreamPublicURL(publicBaseURL, videoIDHex, renditionKey)
}

// RawUploadObjectKey returns the S3 key for the uploaded source file. ext must include the dot (e.g. ".mp4").
func RawUploadObjectKey(videoIDHex, ext string) (string, error) {
	if _, err := primitive.ObjectIDFromHex(videoIDHex); err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidVideoID, err)
	}
	if ext == "" {
		ext = ".bin"
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return fmt.Sprintf("videos/%s/original%s", videoIDHex, ext), nil
}
