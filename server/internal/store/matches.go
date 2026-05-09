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
	// MUST: filter on source state to prevent double-resolve
	filter := bson.M{
		"share_token":    token,
		"state":          "pending",
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

// Complete marks a pending match as completed.
// The filter includes state:"pending" so a tx retry on an already-completed
// document updates 0 rows (idempotent). Returns the number of documents modified
// (0 or 1) so the caller can gate side-effects like IncrementStats.
// ctx should be the session-carrying context from inside a WithTransaction
// callback so the update participates in the caller's transaction.
func (r *MatchRepo) Complete(ctx context.Context, matchID, winnerUID string) (int64, error) {
	oid, err := bson.ObjectIDFromHex(matchID)
	if err != nil {
		return 0, fmt.Errorf("store: Complete: invalid ObjectID %q: %w", matchID, err)
	}
	now := time.Now().UTC()
	// MUST: filter on source state to prevent double-resolve
	filter := bson.M{"_id": oid, "state": "pending"}
	update := bson.M{
		"$set": bson.M{
			"state":        "complete",
			"winner_uid":   winnerUID,
			"completed_at": now,
		},
	}
	res, err := r.coll.UpdateOne(ctx, filter, update)
	if err != nil {
		return 0, fmt.Errorf("store: Complete match %q: %w", matchID, err)
	}
	return res.ModifiedCount, nil
}

// CreateSync inserts a new synchronous match document and returns its hex ID.
// mode is set to "sync", state to "active", started_at to now.
func (r *MatchRepo) CreateSync(ctx context.Context, p1UID, p2UID string, seed int64, gameID string) (matchID string, err error) {
	oid := bson.NewObjectID()
	if err := r.CreateSyncWithID(ctx, oid, p1UID, p2UID, seed, gameID); err != nil {
		return "", err
	}
	return oid.Hex(), nil
}

// CreateSyncWithID inserts a sync match document using a caller-provided
// ObjectID. The handler reserves the ID upfront so both conns can claim
// activeMatchID before the Mongo round-trip — see Phase 09 H2 fix.
func (r *MatchRepo) CreateSyncWithID(ctx context.Context, oid bson.ObjectID, p1UID, p2UID string, seed int64, gameID string) error {
	now := time.Now().UTC()
	m := Match{
		ID:            oid,
		GameID:        gameID,
		Players:       []string{p1UID, p2UID},
		Mode:          "sync",
		State:         "active",
		Seed:          seed,
		CreatedAt:     now,
		SchemaVersion: currentSchemaVersion,
	}
	if _, err := r.coll.InsertOne(ctx, m); err != nil {
		return fmt.Errorf("store: CreateSyncWithID: %w", err)
	}
	return nil
}

// CompleteSync atomically updates a sync match to "complete" and records the
// reason via a Mongo transaction.
// winnerUID may be empty when both players lose (tie-exhaustion or timeout).
// reason is one of: "solved", "exhausted", "forfeit", "timeout".
func (r *MatchRepo) CompleteSync(
	ctx context.Context,
	mongoClient *mongo.Client,
	matchID, winnerUID, reason string,
) error {
	oid, err := bson.ObjectIDFromHex(matchID)
	if err != nil {
		return fmt.Errorf("store: CompleteSync: invalid ObjectID %q: %w", matchID, err)
	}

	session, sErr := mongoClient.StartSession()
	if sErr != nil {
		return fmt.Errorf("store: CompleteSync: StartSession: %w", sErr)
	}
	defer session.EndSession(ctx)

	_, txErr := session.WithTransaction(ctx, func(sc context.Context) (any, error) {
		now := time.Now().UTC()
		// MUST: filter on source state to prevent double-resolve
		// state:"active" ensures re-resolution of already-completed match is a no-op.
		filter := bson.M{"_id": oid, "state": "active"}
		update := bson.M{
			"$set": bson.M{
				"state":        "complete",
				"winner_uid":   winnerUID,
				"completed_at": now,
				"reason":       reason,
			},
		}
		if _, uErr := r.coll.UpdateOne(sc, filter, update); uErr != nil {
			return nil, fmt.Errorf("store: CompleteSync update match: %w", uErr)
		}
		return nil, nil
	})
	return txErr
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
