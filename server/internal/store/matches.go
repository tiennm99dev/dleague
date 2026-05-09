package store

import "go.mongodb.org/mongo-driver/v2/mongo"

// MatchRepo provides access to the `matches` collection.
type MatchRepo struct {
	coll *mongo.Collection
}

// NewMatchRepo returns a MatchRepo backed by the "matches" collection of db.
func NewMatchRepo(db *mongo.Database) *MatchRepo {
	return &MatchRepo{coll: db.Collection("matches")}
}

// TODO(phase-08): Create — insert a new match document; returns assigned ObjectID.
// TODO(phase-08): GetByID — fetch a match by ObjectID string.
// TODO(phase-09): AwardWinner — atomically mark match complete + update both players'
//
//	stats under a MongoDB transaction (session.WithTransaction).
