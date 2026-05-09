// seed-wordlists is a one-shot CLI that upserts the embedded answers.txt and
// dictionary.txt into the Mongo `wordlists` collection. Run once per
// environment to switch from the embedded fallback to the Mongo-backed lists.
//
// Usage:
//
//	DLEAGUE_MONGO_URI=mongodb://localhost:27017 go run ./cmd/seed-wordlists
package main

import (
	"context"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/tiennm99/dleague/server/internal/game/wordle"
	"github.com/tiennm99/dleague/server/internal/store"
)

func main() {
	mongoURI := os.Getenv("DLEAGUE_MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := store.Connect(ctx, mongoURI)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer func() {
		dCtx, dCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer dCancel()
		_ = client.Disconnect(dCtx)
	}()

	db := client.Database()
	coll := db.Collection("wordlists")

	answers := wordle.EmbeddedAnswers()
	dictionary := wordle.EmbeddedDictionary()

	for _, entry := range []struct {
		id    string
		words []string
	}{
		{"wordle_answers", answers},
		{"wordle_dictionary", dictionary},
	} {
		filter := bson.M{"_id": entry.id}
		update := bson.M{
			"$set": bson.M{
				"words":          entry.words,
				"schema_version": 1,
			},
		}
		opts := options.UpdateOne().SetUpsert(true)
		res, err := coll.UpdateOne(ctx, filter, update, opts)
		if err != nil {
			log.Fatalf("upsert %q: %v", entry.id, err)
		}
		log.Printf("seeded %q: upserted=%v matched=%d modified=%d words=%d",
			entry.id, res.UpsertedCount > 0, res.MatchedCount, res.ModifiedCount, len(entry.words))
	}

	log.Println("seed-wordlists: done")
}
