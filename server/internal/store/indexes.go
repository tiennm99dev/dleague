package store

import (
	"context"
	"fmt"
	"log"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// EnsureIndexes creates the application-level indexes across all collections.
// It is idempotent: MongoDB skips creation for indexes with identical key+options.
// Called once at boot after Connect+Ping.
func EnsureIndexes(ctx context.Context, db *mongo.Database) error {
	// Migration safety: refuse to create the unique (match_id, player_uid) index
	// if duplicate documents already exist — auto-deletion would lose data.
	if err := checkAttemptDups(ctx, db); err != nil {
		return err
	}

	type collIndexes struct {
		coll    string
		indexes []mongo.IndexModel
	}

	specs := []collIndexes{
		{
			coll: "users",
			indexes: []mongo.IndexModel{
				// Unique display name — enforces global uniqueness.
				// Note: users._id is the Firebase UID (primary key, auto-unique).
				{
					Keys:    bson.D{{Key: "display_name", Value: 1}},
					Options: options.Index().SetUnique(true).SetName("users_display_name_unique"),
				},
			},
		},
		{
			coll: "matches",
			indexes: []mongo.IndexModel{
				// Array-field index: find all matches a player participated in.
				{
					Keys:    bson.D{{Key: "players", Value: 1}},
					Options: options.Index().SetName("matches_players"),
				},
				// Sort by recency (latest matches first).
				{
					Keys:    bson.D{{Key: "created_at", Value: -1}},
					Options: options.Index().SetName("matches_created_at_desc"),
				},
				// ESR compound: state (equality) + created_at (sort).
				{
					Keys: bson.D{
						{Key: "state", Value: 1},
						{Key: "created_at", Value: -1},
					},
					Options: options.Index().SetName("matches_state_created_at"),
				},
				// Phase 08: unique share_token for O(1) challenge join lookups.
				// Partial filter so Phase 09 sync matches (no share_token / empty
				// string) don't collide on the unique constraint.
				{
					Keys: bson.D{{Key: "share_token", Value: 1}},
					Options: options.Index().
						SetUnique(true).
						SetPartialFilterExpression(bson.M{"share_token": bson.M{"$gt": ""}}).
						SetName("matches_share_token_unique"),
				},
				// Phase 08: sweep expired pending matches efficiently.
				{
					Keys: bson.D{
						{Key: "state", Value: 1},
						{Key: "expires_at", Value: 1},
					},
					Options: options.Index().SetName("matches_state_expires_at"),
				},
			},
		},
		{
			coll: "attempts",
			indexes: []mongo.IndexModel{
				// Find all attempts in a given match.
				{
					Keys:    bson.D{{Key: "match_id", Value: 1}},
					Options: options.Index().SetName("attempts_match_id"),
				},
				// Unique compound: one attempt per player per match.
				// Prevents concurrent-retry duplicate inserts.
				{
					Keys: bson.D{
						{Key: "match_id", Value: 1},
						{Key: "player_uid", Value: 1},
					},
					Options: options.Index().SetUnique(true).SetName("attempts_match_player_unique"),
				},
			},
		},
		{
			coll: "daily_puzzles",
			indexes: []mongo.IndexModel{
				// Date-range queries: recent puzzles sorted descending.
				// Note: daily_puzzles._id is "YYYY-MM-DD" string (unique by primary key).
				{
					Keys:    bson.D{{Key: "_id", Value: -1}},
					Options: options.Index().SetName("daily_puzzles_id_desc"),
				},
			},
		},
		// leaderboards: _id is the primary lookup key ("<game>_<period>_<date>"),
		// auto-indexed by Mongo. No additional indexes needed at MVP.
	}

	total := 0
	for _, s := range specs {
		coll := db.Collection(s.coll)
		res, err := coll.Indexes().CreateMany(ctx, s.indexes)
		if err != nil {
			return fmt.Errorf("store: ensure indexes on %q: %w", s.coll, err)
		}
		total += len(res)
	}

	log.Printf("store: ensured %d indexes", total)
	return nil
}

// checkAttemptDups runs a lightweight aggregation to detect duplicate
// (match_id, player_uid) documents in the attempts collection.
// Returns an error (and logs a clear message) if any duplicates exist so that
// the operator can clean up before the unique index can be created.
func checkAttemptDups(ctx context.Context, db *mongo.Database) error {
	pipeline := mongo.Pipeline{
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "match_id", Value: "$match_id"},
				{Key: "player_uid", Value: "$player_uid"},
			}},
			{Key: "n", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
		{{Key: "$match", Value: bson.D{{Key: "n", Value: bson.D{{Key: "$gt", Value: 1}}}}}},
		{{Key: "$limit", Value: 1}},
	}
	cur, err := db.Collection("attempts").Aggregate(ctx, pipeline)
	if err != nil {
		return fmt.Errorf("store: checkAttemptDups aggregate: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()
	if cur.Next(ctx) {
		log.Printf("store: ERROR refusing to create unique index: duplicate (match_id,player_uid) docs exist; manual cleanup required")
		return fmt.Errorf("store: duplicate (match_id,player_uid) attempts detected; unique index creation aborted")
	}
	return nil
}
