package store

import (
	"context"
	"fmt"
	"log"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// EnsureIndexes creates the 8 application-level indexes across 5 collections.
// It is idempotent: MongoDB skips creation for indexes with identical key+options.
// Called once at boot after Connect+Ping.
func EnsureIndexes(ctx context.Context, db *mongo.Database) error {
	type collIndexes struct {
		coll    string
		indexes []mongo.IndexModel
	}

	specs := []collIndexes{
		{
			coll: "users",
			indexes: []mongo.IndexModel{
				// Unique display name — enforces global uniqueness.
				{
					Keys:    bson.D{{Key: "display_name", Value: 1}},
					Options: options.Index().SetUnique(true),
				},
			},
		},
		{
			coll: "matches",
			indexes: []mongo.IndexModel{
				// Array-field index: find all matches a player participated in.
				{
					Keys: bson.D{{Key: "players", Value: 1}},
				},
				// Sort by recency (latest matches first).
				{
					Keys: bson.D{{Key: "created_at", Value: -1}},
				},
				// ESR compound: state (equality) + created_at (sort).
				{
					Keys: bson.D{
						{Key: "state", Value: 1},
						{Key: "created_at", Value: -1},
					},
				},
			},
		},
		{
			coll: "attempts",
			indexes: []mongo.IndexModel{
				// Find all attempts in a given match.
				{
					Keys: bson.D{{Key: "match_id", Value: 1}},
				},
				// Lookup one player's attempts in a match.
				{
					Keys: bson.D{
						{Key: "match_id", Value: 1},
						{Key: "player_uid", Value: 1},
					},
				},
			},
		},
		{
			coll: "daily_puzzles",
			indexes: []mongo.IndexModel{
				// Date-range queries: recent puzzles sorted descending.
				{
					Keys: bson.D{{Key: "_id", Value: -1}},
				},
			},
		},
		{
			coll: "leaderboards",
			indexes: []mongo.IndexModel{
				// Fetch the latest leaderboard snapshot for a game+period.
				{
					Keys: bson.D{
						{Key: "game_id", Value: 1},
						{Key: "period_end", Value: -1},
					},
				},
			},
		},
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

	log.Printf("store: ensured %d indexes across 5 collections", total)
	return nil
}
