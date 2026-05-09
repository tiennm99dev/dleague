package store

import "go.mongodb.org/mongo-driver/v2/mongo"

// AttemptRepo provides access to the `attempts` collection.
type AttemptRepo struct {
	coll *mongo.Collection
}

// NewAttemptRepo returns an AttemptRepo backed by the "attempts" collection of db.
func NewAttemptRepo(db *mongo.Database) *AttemptRepo {
	return &AttemptRepo{coll: db.Collection("attempts")}
}

// TODO(phase-08): Create — record a player's attempt set for a match.
// TODO(phase-08): ListByMatch — return all attempts for a given match ObjectID.
