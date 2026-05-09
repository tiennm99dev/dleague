// Package main is the dleague admin CLI.
// Usage:
//
//	go run ./server/cmd/admin --action=promote-admin --uid=<firebase-uid>
//	go run ./server/cmd/admin --action=revoke-token  --uid=<firebase-uid>
//
// Environment variables required:
//
//	FIREBASE_PROJECT_ID           — Firebase project ID
//	GOOGLE_APPLICATION_CREDENTIALS — path to service account JSON (or decoded via FIREBASE_SERVICE_ACCOUNT_B64)
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"github.com/tiennm99/dleague/server/internal/auth"
)

func main() {
	action := flag.String("action", "", "promote-admin | revoke-token")
	uid := flag.String("uid", "", "Firebase user UID")
	flag.Parse()

	if *action == "" || *uid == "" {
		flag.Usage()
		os.Exit(1)
	}

	projectID := os.Getenv("FIREBASE_PROJECT_ID")
	credsPath := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	admin, err := auth.NewAdmin(ctx, projectID, credsPath)
	if err != nil {
		log.Fatalf("auth.NewAdmin: %v", err)
	}

	switch *action {
	case "promote-admin":
		if err := admin.SetAdminClaim(ctx, *uid); err != nil {
			log.Fatalf("SetAdminClaim uid=%q: %v", *uid, err)
		}
		log.Printf("OK: admin claim set for uid=%q", *uid)

	case "revoke-token":
		if err := admin.RevokeRefreshTokens(ctx, *uid); err != nil {
			log.Fatalf("RevokeRefreshTokens uid=%q: %v", *uid, err)
		}
		log.Printf("OK: refresh tokens revoked for uid=%q", *uid)

	default:
		log.Fatalf("unknown action %q — use promote-admin or revoke-token", *action)
	}
}
