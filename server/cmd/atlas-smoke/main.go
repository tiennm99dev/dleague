// atlas-smoke connects to the configured MongoDB Atlas cluster, runs a
// {ping: 1} command, and exits 0 on success. Used to validate the
// MONGODB_URI before the full server is wired against it.
//
// Usage:
//
//	MONGODB_URI='mongodb+srv://...' go run ./server/cmd/atlas-smoke
//
// Optional:
//
//	MONGODB_DB=dleague   # database name to ping; default "dleague"
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/tiennm99/dleague/server/internal/store/mongodb"
)

func main() {
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		log.Fatalf("MONGODB_URI is required")
	}
	dbName := os.Getenv("MONGODB_DB")
	if dbName == "" {
		dbName = "dleague"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c, err := mongodb.New(ctx, mongodb.Config{URI: uri, Database: dbName})
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer func() { _ = c.Close() }()

	if err := c.Ping(ctx); err != nil {
		log.Fatalf("ping: %v", err)
	}
	log.Printf("ping ok (db=%s)", dbName)
}
