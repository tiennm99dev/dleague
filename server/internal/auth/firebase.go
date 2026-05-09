// Package auth wraps firebase.google.com/go/v4/auth to provide ID-token
// verification for WebSocket upgrade authentication and admin operations.
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

// Admin wraps auth.Client for privileged operations (custom claims, token revocation).
// Use NewAdmin to construct; keep out of the hot WS path.
type Admin struct {
	client *auth.Client
}

// newApp builds a Firebase app with the given project and credentials.
func newApp(ctx context.Context, projectID, credsPath string) (*firebase.App, error) {
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
	return app, nil
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
	app, err := newApp(ctx, projectID, credsPath)
	if err != nil {
		return nil, err
	}

	client, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("auth: firebase.App.Auth: %w", err)
	}

	return &Verifier{client: client}, nil
}

// NewAdmin initialises an Admin client for privileged operations.
// Requires full service-account credentials (emulator or credsPath/ADC).
func NewAdmin(ctx context.Context, projectID, credsPath string) (*Admin, error) {
	app, err := newApp(ctx, projectID, credsPath)
	if err != nil {
		return nil, err
	}

	client, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("auth: firebase.App.Auth (admin): %w", err)
	}

	return &Admin{client: client}, nil
}

// VerifyIDToken verifies the given Firebase ID token and returns the decoded claims.
// It delegates directly to the underlying auth.Client without revocation checks
// (hot-path cost trade-off; use VerifyIDTokenAndCheckRevoked for admin/moderator ops).
func (v *Verifier) VerifyIDToken(ctx context.Context, idToken string) (*auth.Token, error) {
	token, err := v.client.VerifyIDToken(ctx, idToken)
	if err != nil {
		return nil, fmt.Errorf("auth: VerifyIDToken: %w", err)
	}
	return token, nil
}

// SetAdminClaim sets the "admin": true custom claim on a Firebase user.
// The change takes effect on the user's next token refresh (~1 hour).
// Use the admin CLI (cmd/admin) to call this.
func (a *Admin) SetAdminClaim(ctx context.Context, uid string) error {
	claims := map[string]interface{}{"admin": true}
	if err := a.client.SetCustomUserClaims(ctx, uid, claims); err != nil {
		return fmt.Errorf("auth: SetCustomUserClaims uid=%q: %w", uid, err)
	}
	return nil
}

// RevokeRefreshTokens invalidates all existing refresh tokens for the user.
// Any active ID tokens remain valid until their 1-hour expiry; pair with
// VerifyIDTokenAndCheckRevoked on sensitive operations to enforce immediate revocation.
func (a *Admin) RevokeRefreshTokens(ctx context.Context, uid string) error {
	if err := a.client.RevokeRefreshTokens(ctx, uid); err != nil {
		return fmt.Errorf("auth: RevokeRefreshTokens uid=%q: %w", uid, err)
	}
	return nil
}

// VerifyIDTokenAndCheckRevoked verifies an ID token AND checks the revocation
// status against Firebase. More expensive than VerifyIDToken (requires a network
// round-trip to Firebase). Use only for admin/moderator-action handlers, not the
// normal WS hot path.
func (a *Admin) VerifyIDTokenAndCheckRevoked(ctx context.Context, idToken string) (*auth.Token, error) {
	token, err := a.client.VerifyIDTokenAndCheckRevoked(ctx, idToken)
	if err != nil {
		return nil, fmt.Errorf("auth: VerifyIDTokenAndCheckRevoked: %w", err)
	}
	return token, nil
}
