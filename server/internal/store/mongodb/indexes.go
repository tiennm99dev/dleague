package mongodb

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Collection names live as constants so misspellings surface at compile time.
const (
	collUsers        = "users"
	collPuzzles      = "puzzles"
	collAttempts     = "attempts"
	collMatches      = "matches"
	collLeaderboards = "leaderboards"
	collPresence     = "presence"
	collCache        = "cache"
)

// indexSpec is one (collection, index) pair.
type indexSpec struct {
	collection string
	model      mongo.IndexModel
}

// indexes returns every index dleague needs.
//
// TTL caveat (presence + cache): the indexed `expireAt` field MUST be a Go
// `time.Time` value. The Mongo Go driver encodes that as BSON Date and the
// background TTL monitor only purges Date-typed fields. Strings or epoch
// ints are silently retained forever.
func indexes() []indexSpec {
	specs := []indexSpec{
		// users — uid unique. _id is Mongo-generated; uid is the lookup key.
		{collUsers, mongo.IndexModel{
			Keys:    bson.D{{Key: "uid", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("uid_unique"),
		}},
		// puzzles — _id is the date string (set via bson:"_id" on Puzzle.Date).
		// No extra index needed; _id is automatically unique.

		// attempts — compound unique on (uid, puzzleDate). Mongo-generated _id.
		{collAttempts, mongo.IndexModel{
			Keys: bson.D{
				{Key: "uid", Value: 1},
				{Key: "puzzleDate", Value: 1},
			},
			Options: options.Index().SetUnique(true).SetName("uid_puzzleDate_unique"),
		}},
		// matches — _id is the natural match ID. Index for ListUserMatches:
		// "find matches that include uid in players, sort by createdAt desc".
		{collMatches, mongo.IndexModel{
			Keys: bson.D{
				{Key: "players", Value: 1},
				{Key: "createdAt", Value: -1},
			},
			Options: options.Index().SetName("players_createdAt"),
		}},
		// leaderboards — sort index for TopN, plus unique compound for
		// upsert filter parity with Redis ZADD GT semantics.
		{collLeaderboards, mongo.IndexModel{
			Keys: bson.D{
				{Key: "board", Value: 1},
				{Key: "score", Value: -1},
			},
			Options: options.Index().SetName("board_score_desc"),
		}},
		{collLeaderboards, mongo.IndexModel{
			Keys: bson.D{
				{Key: "board", Value: 1},
				{Key: "uid", Value: 1},
			},
			Options: options.Index().SetUnique(true).SetName("board_uid_unique"),
		}},
		// presence — TTL on expireAt. expireAfterSeconds=0 means
		// "purge when expireAt <= now". Background scan runs ~every 60s,
		// so reads must always include `expireAt: {$gt: now()}` to mask
		// the up-to-90s purge lag.
		{collPresence, mongo.IndexModel{
			Keys:    bson.D{{Key: "expireAt", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0).SetName("expireAt_ttl"),
		}},
		// cache — same TTL pattern as presence. Same query-side filter rule.
		{collCache, mongo.IndexModel{
			Keys:    bson.D{{Key: "expireAt", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0).SetName("expireAt_ttl"),
		}},
	}
	return specs
}

// ensureIndexes is idempotent: Mongo no-ops on duplicate index specs.
// On a malformed spec, prior indexes in the slice are still created and the
// bad one returns the error — re-running ensureIndexes is safe.
func ensureIndexes(ctx context.Context, db *mongo.Database) error {
	for _, spec := range indexes() {
		coll := db.Collection(spec.collection)
		if _, err := coll.Indexes().CreateOne(ctx, spec.model); err != nil {
			return fmt.Errorf("mongodb: create index on %s: %w", spec.collection, err)
		}
	}
	return nil
}
