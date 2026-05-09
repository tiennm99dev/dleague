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

// DailyPuzzleRepo provides access to the `daily_puzzles` collection.
type DailyPuzzleRepo struct {
	coll *mongo.Collection
}

// NewDailyPuzzleRepo returns a DailyPuzzleRepo backed by the "daily_puzzles" collection of db.
func NewDailyPuzzleRepo(db *mongo.Database) *DailyPuzzleRepo {
	return &DailyPuzzleRepo{coll: db.Collection("daily_puzzles")}
}

// GetByDate fetches the daily puzzle for the given "YYYY-MM-DD" date key.
// Returns (nil, nil) when no document exists for that date.
func (r *DailyPuzzleRepo) GetByDate(ctx context.Context, date string) (*DailyPuzzle, error) {
	if date == "" {
		return nil, fmt.Errorf("store: GetByDate: date must not be empty")
	}
	var p DailyPuzzle
	err := r.coll.FindOne(ctx, bson.M{"_id": date}).Decode(&p)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("store: GetByDate %q: %w", date, err)
	}
	return &p, nil
}

// Upsert inserts or replaces the daily puzzle document identified by p.ID.
// Sets created_at only on insert (via $setOnInsert) so re-inserts are idempotent.
func (r *DailyPuzzleRepo) Upsert(ctx context.Context, p DailyPuzzle) error {
	if p.ID == "" {
		return fmt.Errorf("store: Upsert DailyPuzzle: ID must not be empty")
	}
	now := time.Now().UTC()
	filter := bson.M{"_id": p.ID}
	update := bson.M{
		"$set": bson.M{
			"game_id":        p.GameID,
			"seed":           p.Seed,
			"solution":       p.Solution,
			"solution_hash":  p.SolutionHash,
			"difficulty":     p.Difficulty,
			"schema_version": p.SchemaVersion,
		},
		"$setOnInsert": bson.M{
			"created_at": now,
		},
	}
	opts := options.UpdateOne().SetUpsert(true)
	_, err := r.coll.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("store: Upsert DailyPuzzle %q: %w", p.ID, err)
	}
	return nil
}
