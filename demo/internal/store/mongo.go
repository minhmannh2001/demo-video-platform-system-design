package store

import (
	"context"
	"errors"
	"time"

	"video-platform/demo/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

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
	doc := bson.M{
		"_id":         oid,
		"title":       v.Title,
		"description": v.Description,
		"uploader":    v.Uploader,
		"raw_s3_key":  v.RawS3Key,
		"status":      v.Status,
		"created_at":  v.CreatedAt,
		"updated_at":  v.UpdatedAt,
	}
	_, err = s.coll.InsertOne(ctx, doc)
	return err
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
	return &models.Video{
		ID:            raw.ID.Hex(),
		Title:         raw.Title,
		Description:   raw.Description,
		Uploader:      raw.Uploader,
		RawS3Key:      raw.RawS3Key,
		EncodedPrefix: raw.EncodedPrefix,
		Status:        raw.Status,
		DurationSec:   raw.DurationSec,
		CreatedAt:     raw.CreatedAt,
		UpdatedAt:     raw.UpdatedAt,
	}, nil
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
		out = append(out, models.Video{
			ID:            raw.ID.Hex(),
			Title:         raw.Title,
			Description:   raw.Description,
			Uploader:      raw.Uploader,
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
