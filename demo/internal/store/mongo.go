package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"video-platform/demo/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ErrVideoNotFound is returned when an update targets a missing video id.
var ErrVideoNotFound = errors.New("store: video not found")

type VideoStore struct {
	coll *mongo.Collection
}

func Connect(ctx context.Context, uri string) (*mongo.Client, error) {
	return mongo.Connect(ctx, options.Client().ApplyURI(uri))
}

func NewVideoStore(db *mongo.Database) *VideoStore {
	return &VideoStore{coll: db.Collection("videos")}
}

func (s *VideoStore) Create(ctx context.Context, v *models.Video) error {
	oid, err := primitive.ObjectIDFromHex(v.ID)
	if err != nil {
		return err
	}
	v.CreatedAt = time.Now().UTC()
	v.UpdatedAt = v.CreatedAt
	vis := v.Visibility
	if vis == "" {
		vis = models.VisibilityPublic
		v.Visibility = vis
	}
	if !models.ValidVisibility(vis) {
		return fmt.Errorf("store: invalid visibility %q", vis)
	}
	doc := bson.M{
		"_id":         oid,
		"title":       v.Title,
		"description": v.Description,
		"uploader":    v.Uploader,
		"visibility":  vis,
		"raw_s3_key":  v.RawS3Key,
		"status":      v.Status,
		"created_at":  v.CreatedAt,
		"updated_at":  v.UpdatedAt,
	}
	_, err = s.coll.InsertOne(ctx, doc)
	return err
}

// UpdateMetadata sets user-editable fields used for catalog and search indexing.
// Title must be non-empty. Pass visibility=="" to leave visibility unchanged.
func (s *VideoStore) UpdateMetadata(ctx context.Context, id, title, description, visibility string) error {
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("store: title required")
	}
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	set := bson.M{
		"title":       title,
		"description": description,
		"updated_at":  time.Now().UTC(),
	}
	if visibility != "" {
		if !models.ValidVisibility(visibility) {
			return fmt.Errorf("store: invalid visibility %q", visibility)
		}
		set["visibility"] = visibility
	}
	res, err := s.coll.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{"$set": set})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return ErrVideoNotFound
	}
	return nil
}

func (s *VideoStore) GetByID(ctx context.Context, id string) (*models.Video, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var raw struct {
		ID            primitive.ObjectID `bson:"_id"`
		Title         string             `bson:"title"`
		Description   string             `bson:"description"`
		Uploader      string             `bson:"uploader"`
		Visibility    string             `bson:"visibility"`
		RawS3Key      string             `bson:"raw_s3_key"`
		EncodedPrefix string             `bson:"encoded_prefix"`
		Status        string             `bson:"status"`
		DurationSec   int                `bson:"duration_sec"`
		CreatedAt     time.Time          `bson:"created_at"`
		UpdatedAt     time.Time          `bson:"updated_at"`
	}
	err = s.coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&raw)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	v := &models.Video{
		ID:            raw.ID.Hex(),
		Title:         raw.Title,
		Description:   raw.Description,
		Uploader:      raw.Uploader,
		Visibility:    raw.Visibility,
		RawS3Key:      raw.RawS3Key,
		EncodedPrefix: raw.EncodedPrefix,
		Status:        raw.Status,
		DurationSec:   raw.DurationSec,
		CreatedAt:     raw.CreatedAt,
		UpdatedAt:     raw.UpdatedAt,
	}
	if v.Visibility == "" {
		v.Visibility = models.VisibilityPublic
	}
	return v, nil
}

func (s *VideoStore) List(ctx context.Context, limit int64) ([]models.Video, error) {
	if limit <= 0 {
		limit = 50
	}
	opts := options.Find().SetSort(bson.D{{Key: "_id", Value: -1}}).SetLimit(limit)
	cur, err := s.coll.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []models.Video
	for cur.Next(ctx) {
		var raw struct {
			ID            primitive.ObjectID `bson:"_id"`
			Title         string             `bson:"title"`
			Description   string             `bson:"description"`
			Uploader      string             `bson:"uploader"`
			Visibility    string             `bson:"visibility"`
			RawS3Key      string             `bson:"raw_s3_key"`
			EncodedPrefix string             `bson:"encoded_prefix"`
			Status        string             `bson:"status"`
			DurationSec   int                `bson:"duration_sec"`
			CreatedAt     time.Time          `bson:"created_at"`
			UpdatedAt     time.Time          `bson:"updated_at"`
		}
		if err := cur.Decode(&raw); err != nil {
			return nil, err
		}
		vis := raw.Visibility
		if vis == "" {
			vis = models.VisibilityPublic
		}
		out = append(out, models.Video{
			ID:            raw.ID.Hex(),
			Title:         raw.Title,
			Description:   raw.Description,
			Uploader:      raw.Uploader,
			Visibility:    vis,
			RawS3Key:      raw.RawS3Key,
			EncodedPrefix: raw.EncodedPrefix,
			Status:        raw.Status,
			DurationSec:   raw.DurationSec,
			CreatedAt:     raw.CreatedAt,
			UpdatedAt:     raw.UpdatedAt,
		})
	}
	if err := cur.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		return []models.Video{}, nil
	}
	return out, nil
}

// ForEachVideoBatch streams all videos ordered by _id using a Mongo cursor. fn receives batches of at most batchSize
// items (the last batch may be smaller). Intended for one-off backfill jobs.
func (s *VideoStore) ForEachVideoBatch(ctx context.Context, batchSize int, fn func(context.Context, []models.Video) error) error {
	if batchSize <= 0 {
		batchSize = 500
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "_id", Value: 1}}).
		SetBatchSize(int32(batchSize))
	cur, err := s.coll.Find(ctx, bson.M{}, opts)
	if err != nil {
		return err
	}
	defer cur.Close(ctx)

	batch := make([]models.Video, 0, batchSize)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if err := fn(ctx, batch); err != nil {
			return err
		}
		batch = batch[:0]
		return nil
	}

	for cur.Next(ctx) {
		var raw struct {
			ID            primitive.ObjectID `bson:"_id"`
			Title         string             `bson:"title"`
			Description   string             `bson:"description"`
			Uploader      string             `bson:"uploader"`
			Visibility    string             `bson:"visibility"`
			RawS3Key      string             `bson:"raw_s3_key"`
			EncodedPrefix string             `bson:"encoded_prefix"`
			Status        string             `bson:"status"`
			DurationSec   int                `bson:"duration_sec"`
			CreatedAt     time.Time          `bson:"created_at"`
			UpdatedAt     time.Time          `bson:"updated_at"`
		}
		if err := cur.Decode(&raw); err != nil {
			return err
		}
		vis := raw.Visibility
		if vis == "" {
			vis = models.VisibilityPublic
		}
		batch = append(batch, models.Video{
			ID:            raw.ID.Hex(),
			Title:         raw.Title,
			Description:   raw.Description,
			Uploader:      raw.Uploader,
			Visibility:    vis,
			RawS3Key:      raw.RawS3Key,
			EncodedPrefix: raw.EncodedPrefix,
			Status:        raw.Status,
			DurationSec:   raw.DurationSec,
			CreatedAt:     raw.CreatedAt,
			UpdatedAt:     raw.UpdatedAt,
		})
		if len(batch) >= batchSize {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := cur.Err(); err != nil {
		return err
	}
	return flush()
}

func (s *VideoStore) MarkReady(ctx context.Context, id, encodedPrefix string, durationSec int) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	_, err = s.coll.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{
		"$set": bson.M{
			"status":         models.StatusReady,
			"encoded_prefix": encodedPrefix,
			"duration_sec":   durationSec,
			"updated_at":     now,
		},
	})
	return err
}

func (s *VideoStore) MarkFailed(ctx context.Context, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	_, err = s.coll.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{
		"$set": bson.M{
			"status":     models.StatusFailed,
			"updated_at": time.Now().UTC(),
		},
	})
	return err
}

// DeleteByID removes the video document. Returns false if no document matched (e.g. wrong id).
func (s *VideoStore) DeleteByID(ctx context.Context, id string) (bool, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return false, err
	}
	res, err := s.coll.DeleteOne(ctx, bson.M{"_id": oid})
	if err != nil {
		return false, err
	}
	return res.DeletedCount > 0, nil
}
