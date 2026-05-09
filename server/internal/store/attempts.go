package store

// In mongo-driver v2, sessions are embedded in a plain context.Context via
// mongo.NewSessionContext. Methods accepting a session ctx take a single
// context.Context; callers inside WithTransaction pass the callback's ctx.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// ErrAttemptExists is returned by Insert when a player has already submitted
// an attempt for the given match (idempotency guard).
var ErrAttemptExists = errors.New("store: attempt already exists for this player+match")

// AttemptRepo provides access to the `attempts` collection.
type AttemptRepo struct {
	coll *mongo.Collection
}

// NewAttemptRepo returns an AttemptRepo backed by the "attempts" collection of db.
func NewAttemptRepo(db *mongo.Database) *AttemptRepo {
	return &AttemptRepo{coll: db.Collection("attempts")}
}

// GetByMatchAndPlayer fetches an existing attempt by (match_id, player_uid).
// Returns (nil, nil) when no document exists.
func (r *AttemptRepo) GetByMatchAndPlayer(ctx context.Context, matchID, uid string) (*Attempt, error) {
	oid, err := bson.ObjectIDFromHex(matchID)
	if err != nil {
		return nil, fmt.Errorf("store: GetByMatchAndPlayer: invalid matchID %q: %w", matchID, err)
	}
	var a Attempt
	err = r.coll.FindOne(ctx, bson.M{"match_id": oid, "player_uid": uid}).Decode(&a)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("store: GetByMatchAndPlayer: %w", err)
	}
	return &a, nil
}

// Insert records a player's attempt for a match.
// ctx should be the session-carrying context from inside a WithTransaction
// callback when transactional behaviour is required.
// Returns ErrAttemptExists if the (match_id, player_uid) pair already exists.
func (r *AttemptRepo) Insert(ctx context.Context, a Attempt) error {
	// Idempotency check inside the provided context (participates in any tx).
	var existing Attempt
	checkErr := r.coll.FindOne(ctx, bson.M{
		"match_id":   a.MatchID,
		"player_uid": a.PlayerUID,
	}).Decode(&existing)
	if checkErr == nil {
		return ErrAttemptExists
	}
	if !errors.Is(checkErr, mongo.ErrNoDocuments) {
		return fmt.Errorf("store: Insert attempt idempotency check: %w", checkErr)
	}

	a.CreatedAt = time.Now().UTC()
	a.SchemaVersion = currentSchemaVersion
	_, err := r.coll.InsertOne(ctx, a)
	if err != nil {
		// Unique-index violation (code 11000): treat as idempotent — the attempt
		// was already inserted by a concurrent tx attempt. Surface ErrAttemptExists
		// so the caller can skip re-processing rather than returning a 500.
		var we mongo.WriteException
		if errors.As(err, &we) {
			for _, w := range we.WriteErrors {
				if w.Code == 11000 {
					return ErrAttemptExists
				}
			}
		}
		return fmt.Errorf("store: Insert attempt: %w", err)
	}
	return nil
}

// ListByMatch returns all attempts for the given match ObjectID hex string.
func (r *AttemptRepo) ListByMatch(ctx context.Context, matchID string) ([]Attempt, error) {
	oid, err := bson.ObjectIDFromHex(matchID)
	if err != nil {
		return nil, fmt.Errorf("store: ListByMatch: invalid matchID %q: %w", matchID, err)
	}
	cur, err := r.coll.Find(ctx, bson.M{"match_id": oid})
	if err != nil {
		return nil, fmt.Errorf("store: ListByMatch: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()

	var attempts []Attempt
	if err := cur.All(ctx, &attempts); err != nil {
		return nil, fmt.Errorf("store: ListByMatch decode: %w", err)
	}
	return attempts, nil
}
