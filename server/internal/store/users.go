package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ErrEmptyUID is returned when an empty Firebase UID is passed to any UserRepo method.
var ErrEmptyUID = errors.New("store: uid must not be empty")

// UserProfile contains the mutable fields set on login / registration.
// Used as the upsert payload so callers don't construct a full User.
type UserProfile struct {
	DisplayName string
	AvatarURL   string
	Verified    bool
}

// UserRepo provides CRUD operations on the `users` collection.
type UserRepo struct {
	coll *mongo.Collection
}

// NewUserRepo returns a UserRepo backed by the "users" collection of db.
func NewUserRepo(db *mongo.Database) *UserRepo {
	return &UserRepo{coll: db.Collection("users")}
}

// UpsertByUID creates or updates a user document keyed by Firebase UID.
// On insert, CreatedAt and Stats are set to zero/defaults with schema_version=1.
// On update, only display_name, avatar_url, verified, and last_login are touched.
// Returns ErrEmptyUID if uid is empty.
func (r *UserRepo) UpsertByUID(ctx context.Context, uid string, p UserProfile) error {
	if uid == "" {
		return ErrEmptyUID
	}

	now := time.Now().UTC()

	filter := bson.M{"_id": uid}
	update := bson.M{
		"$set": bson.M{
			"display_name":   p.DisplayName,
			"avatar_url":     p.AvatarURL,
			"verified":       p.Verified,
			"last_login":     now,
			"schema_version": currentSchemaVersion,
		},
		"$setOnInsert": bson.M{
			"created_at": now,
			"stats": bson.M{
				"wins":           0,
				"losses":         0,
				"current_streak": 0,
				"total_games":    0,
			},
		},
	}

	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	res := r.coll.FindOneAndUpdate(ctx, filter, update, opts)
	if res.Err() != nil && !errors.Is(res.Err(), mongo.ErrNoDocuments) {
		return fmt.Errorf("store: upsert user %q: %w", uid, res.Err())
	}
	return nil
}

// GetByUID fetches a user by Firebase UID.
// Returns (nil, nil) when the document does not exist (ErrNoDocuments is swallowed).
// Returns ErrEmptyUID if uid is empty.
func (r *UserRepo) GetByUID(ctx context.Context, uid string) (*User, error) {
	if uid == "" {
		return nil, ErrEmptyUID
	}

	var u User
	err := r.coll.FindOne(ctx, bson.M{"_id": uid}).Decode(&u)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("store: get user %q: %w", uid, err)
	}
	return &u, nil
}
