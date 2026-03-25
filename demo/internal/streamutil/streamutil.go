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

// ContentTypeByFilename picks a Content-Type for HLS segments by file extension.
func ContentTypeByFilename(filename string) string {
	switch strings.ToLower(path.Ext(filename)) {
	case ".m3u8":
		return "application/vnd.apple.mpegurl"
	case ".ts":
		return "video/mp2t"
	default:
		return "application/octet-stream"
	}
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
	base := strings.TrimRight(publicBaseURL, "/")
	return fmt.Sprintf("%s/stream/%s/master.m3u8", base, videoIDHex)
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
