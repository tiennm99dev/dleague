package store

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// GameRepo provides access to the `games` collection (game-type registry).
type GameRepo struct {
	coll *mongo.Collection
}

// NewGameRepo returns a GameRepo backed by the "games" collection of db.
func NewGameRepo(db *mongo.Database) *GameRepo {
	return &GameRepo{coll: db.Collection("games")}
}

// Get fetches a game by its slug ID (e.g. "wordle").
// Returns (nil, nil) when the document does not exist.
func (r *GameRepo) Get(ctx context.Context, gameID string) (*Game, error) {
	if gameID == "" {
		return nil, fmt.Errorf("store: gameID must not be empty")
	}
	var g Game
	err := r.coll.FindOne(ctx, bson.M{"_id": gameID}).Decode(&g)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("store: get game %q: %w", gameID, err)
	}
	return &g, nil
}

// TODO(phase-07): RegisterGame — insert or replace a game registry entry.
// TODO(phase-07): ListGames — return all active games.
