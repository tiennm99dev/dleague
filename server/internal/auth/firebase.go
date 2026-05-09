// Package auth wraps firebase.google.com/go/v4/auth to provide ID-token
// verification for WebSocket upgrade authentication.
//
// Credential resolution order (first matching wins):
//  1. FIREBASE_AUTH_EMULATOR_HOST env set → SDK auto-routes to local emulator; no creds needed.
//  2. credsPath non-empty → option.WithCredentialsFile(credsPath).
//  3. Default Application Default Credentials (GOOGLE_APPLICATION_CREDENTIALS env or
//     Workload Identity on Fly.io).
package auth

import (
	"context"
	"errors"
	"fmt"
	"os"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

// ErrMissingProjectID is returned by New when projectID is empty and the
// emulator is not active (the SDK needs a project ID to validate token audience).
var ErrMissingProjectID = errors.New("auth: FIREBASE_PROJECT_ID is required when not using the emulator")

// Verifier wraps auth.Client and exposes VerifyIDToken.
type Verifier struct {
	client *auth.Client
}

// New initialises a Verifier for the given Firebase project.
//
//   - If FIREBASE_AUTH_EMULATOR_HOST is set the SDK auto-routes to the local emulator.
//     In that case credentials are optional and the function succeeds with any non-empty projectID.
//   - Otherwise credsPath (path to service-account JSON) is used when non-empty;
//     default Application Default Credentials are used when empty.
//
// Returns ErrMissingProjectID when projectID is empty outside emulator mode.
func New(ctx context.Context, projectID, credsPath string) (*Verifier, error) {
	usingEmulator := os.Getenv("FIREBASE_AUTH_EMULATOR_HOST") != ""

	if projectID == "" && !usingEmulator {
		return nil, ErrMissingProjectID
	}

	conf := &firebase.Config{}
	if projectID != "" {
		conf.ProjectID = projectID
	}

	var opts []option.ClientOption
	if !usingEmulator && credsPath != "" {
		opts = append(opts, option.WithCredentialsFile(credsPath))
	}

	app, err := firebase.NewApp(ctx, conf, opts...)
	if err != nil {
		return nil, fmt.Errorf("auth: firebase.NewApp: %w", err)
	}

	client, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("auth: firebase.App.Auth: %w", err)
	}

	return &Verifier{client: client}, nil
}

// VerifyIDToken verifies the given Firebase ID token and returns the decoded claims.
// It delegates directly to the underlying auth.Client without revocation checks
// (revocation check deferred to Phase 10 — see phase-05 spec §key-insights).
func (v *Verifier) VerifyIDToken(ctx context.Context, idToken string) (*auth.Token, error) {
	token, err := v.client.VerifyIDToken(ctx, idToken)
	if err != nil {
		return nil, fmt.Errorf("auth: VerifyIDToken: %w", err)
	}
	return token, nil
}
