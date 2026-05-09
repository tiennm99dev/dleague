package store

// In mongo-driver v2, sessions are embedded in a plain context.Context via
// mongo.NewSessionContext. Methods that need transaction participation accept a
// single context.Context; callers inside a WithTransaction callback pass the
// session-carrying ctx they receive from the callback.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ErrAlreadyJoined is returned by JoinAsChallengee when the match already has
// a challengee (concurrent join race lost) or the share token is not found.
var ErrAlreadyJoined = errors.New("store: match already joined or not found")

// MatchRepo provides access to the `matches` collection.
type MatchRepo struct {
	coll *mongo.Collection
}

// NewMatchRepo returns a MatchRepo backed by the "matches" collection of db.
func NewMatchRepo(db *mongo.Database) *MatchRepo {
	return &MatchRepo{coll: db.Collection("matches")}
}

// Create inserts a new async match document.
// share_token is generated internally (UUID v4) and returned alongside the
// newly assigned ObjectID hex string. expires_at is set to now+7 days.
func (r *MatchRepo) Create(ctx context.Context, m Match) (matchID, shareToken string, err error) {
	now := time.Now().UTC()
	token := uuid.NewString()

	m.ShareToken = token
	m.State = "pending"
	m.Mode = "async"
	m.CreatedAt = now
	m.ExpiresAt = now.Add(7 * 24 * time.Hour)
	m.SchemaVersion = currentSchemaVersion

	res, err := r.coll.InsertOne(ctx, m)
	if err != nil {
		return "", "", fmt.Errorf("store: create match: %w", err)
	}
	oid, ok := res.InsertedID.(bson.ObjectID)
	if !ok {
		return "", "", fmt.Errorf("store: create match: unexpected InsertedID type")
	}
	return oid.Hex(), token, nil
}

// GetByShareToken fetches a match by its share token.
// Returns (nil, nil) when no document matches (ErrNoDocuments swallowed).
func (r *MatchRepo) GetByShareToken(ctx context.Context, token string) (*Match, error) {
	var m Match
	err := r.coll.FindOne(ctx, bson.M{"share_token": token}).Decode(&m)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("store: GetByShareToken: %w", err)
	}
	return &m, nil
}

// GetByID fetches a match by its ObjectID hex string.
// Returns (nil, nil) when no document matches.
func (r *MatchRepo) GetByID(ctx context.Context, matchID string) (*Match, error) {
	oid, err := bson.ObjectIDFromHex(matchID)
	if err != nil {
		return nil, fmt.Errorf("store: GetByID: invalid ObjectID %q: %w", matchID, err)
	}
	var m Match
	if err := r.coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&m); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("store: GetByID %q: %w", matchID, err)
	}
	return &m, nil
}

// JoinAsChallengee atomically sets challengee_uid on the match identified by
// share_token, but only when challengee_uid is currently unset.
// ctx should be the session-carrying context from inside a WithTransaction
// callback so the update participates in the caller's transaction.
// Returns ErrAlreadyJoined if the token does not exist or is already taken.
func (r *MatchRepo) JoinAsChallengee(ctx context.Context, token, uid string) (*Match, error) {
	now := time.Now().UTC()
	filter := bson.M{
		"share_token":    token,
		"challengee_uid": bson.M{"$exists": false},
	}
	update := bson.M{
		"$set": bson.M{
			"challengee_uid": uid,
			"joined_at":      now,
		},
	}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	var m Match
	err := r.coll.FindOneAndUpdate(ctx, filter, update, opts).Decode(&m)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrAlreadyJoined
		}
		return nil, fmt.Errorf("store: JoinAsChallengee: %w", err)
	}
	return &m, nil
}

// Complete marks a match as completed.
// ctx should be the session-carrying context from inside a WithTransaction
// callback so the update participates in the caller's transaction.
func (r *MatchRepo) Complete(ctx context.Context, matchID, winnerUID string) error {
	oid, err := bson.ObjectIDFromHex(matchID)
	if err != nil {
		return fmt.Errorf("store: Complete: invalid ObjectID %q: %w", matchID, err)
	}
	now := time.Now().UTC()
	update := bson.M{
		"$set": bson.M{
			"state":        "complete",
			"winner_uid":   winnerUID,
			"completed_at": now,
		},
	}
	_, err = r.coll.UpdateOne(ctx, bson.M{"_id": oid}, update)
	if err != nil {
		return fmt.Errorf("store: Complete match %q: %w", matchID, err)
	}
	return nil
}

// SweepExpired deletes pending matches whose expires_at is in the past.
// Best-effort cleanup; safe to call outside a transaction.
func (r *MatchRepo) SweepExpired(ctx context.Context) error {
	filter := bson.M{
		"state":      "pending",
		"expires_at": bson.M{"$lt": time.Now().UTC()},
	}
	_, err := r.coll.DeleteMany(ctx, filter)
	if err != nil {
		return fmt.Errorf("store: SweepExpired: %w", err)
	}
	return nil
}
