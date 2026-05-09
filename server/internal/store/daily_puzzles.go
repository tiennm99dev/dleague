package store

import "go.mongodb.org/mongo-driver/v2/mongo"

// DailyPuzzleRepo provides access to the `daily_puzzles` collection.
type DailyPuzzleRepo struct {
	coll *mongo.Collection
}

// NewDailyPuzzleRepo returns a DailyPuzzleRepo backed by the "daily_puzzles" collection of db.
func NewDailyPuzzleRepo(db *mongo.Database) *DailyPuzzleRepo {
	return &DailyPuzzleRepo{coll: db.Collection("daily_puzzles")}
}

// TODO(phase-07): GetByDate — fetch puzzle by "YYYY-MM-DD" _id key.
// TODO(phase-07): Upsert — create or replace a daily puzzle for a given date.
