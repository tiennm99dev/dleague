// dleague-export streams every persistent doc as JSONL to stdout.
//
// Migration escape hatch: pipe the output to any importer that knows the
// shape `{"collection":"<name>","doc":{...}}`. The doc fields match
// store.User / Puzzle / Attempt / Match in the corresponding collection.
//
// Usage:
//
//	dleague-export > snapshot.jsonl
//
// Reads the same env vars as the server: COUCHBASE_CONN_STRING,
// COUCHBASE_USERNAME, COUCHBASE_PASSWORD, COUCHBASE_BUCKET. Redis state
// (leaderboards, presence, cache) is intentionally not exported — it is
// derivable from the Couchbase attempts.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"github.com/tiennm99/dleague/server/internal/store/couchbase"
)

func main() {
	timeoutFlag := flag.Duration("timeout", 5*time.Minute, "overall export timeout")
	flag.Parse()

	cfg := couchbase.Config{
		ConnString: getEnv("COUCHBASE_CONN_STRING"),
		Username:   getEnv("COUCHBASE_USERNAME"),
		Password:   getEnv("COUCHBASE_PASSWORD"),
		Bucket:     getEnv("COUCHBASE_BUCKET"),
	}

	bootCtx, cancelBoot := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelBoot()

	client, err := couchbase.New(bootCtx, cfg)
	if err != nil {
		log.Fatalf("couchbase: %v", err)
	}
	defer client.Close()

	exportCtx, cancel := context.WithTimeout(context.Background(), *timeoutFlag)
	defer cancel()

	if err := client.Export(exportCtx, os.Stdout); err != nil {
		log.Fatalf("export: %v", err)
	}
}

func getEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing env: %s", key)
	}
	return v
}
