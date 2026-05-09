package store

import "go.mongodb.org/mongo-driver/v2/mongo"

// LeaderboardRepo provides access to the `leaderboards` collection.
type LeaderboardRepo struct {
	coll *mongo.Collection
}

// NewLeaderboardRepo returns a LeaderboardRepo backed by the "leaderboards" collection of db.
func NewLeaderboardRepo(db *mongo.Database) *LeaderboardRepo {
	return &LeaderboardRepo{coll: db.Collection("leaderboards")}
}

// TODO(phase-08): GetLatest — fetch the most recent snapshot for a game+period.
// TODO(phase-08): Upsert — write a pre-computed leaderboard snapshot.
