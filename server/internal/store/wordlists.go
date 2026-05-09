package store

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// wordlistDoc is the Mongo document shape for the `wordlists` collection.
// _id is a stable string key such as "wordle_answers" or "wordle_dictionary".
type wordlistDoc struct {
	ID            string   `bson:"_id"`
	Words         []string `bson:"words"`
	SchemaVersion int      `bson:"schema_version"`
}

// WordlistRepo provides read access to the `wordlists` collection.
// Word lists are seeded once via `make seed-wordlists`; this repo is read-only
// at runtime. The embedded fallback in wordlist.go handles an empty collection.
type WordlistRepo struct {
	coll *mongo.Collection
}

// NewWordlistRepo returns a WordlistRepo backed by the "wordlists" collection of db.
func NewWordlistRepo(db *mongo.Database) *WordlistRepo {
	return &WordlistRepo{coll: db.Collection("wordlists")}
}

// GetByID fetches the word slice for the given list ID.
// Returns an empty slice (not an error) when the document does not exist —
// callers fall back to the embedded word list in that case.
func (r *WordlistRepo) GetByID(ctx context.Context, id string) ([]string, error) {
	if id == "" {
		return nil, fmt.Errorf("store: WordlistRepo.GetByID: id must not be empty")
	}
	var doc wordlistDoc
	err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("store: WordlistRepo.GetByID %q: %w", id, err)
	}
	return doc.Words, nil
}
