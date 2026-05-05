package auth

import (
	"context"
	"fmt"

	firebase "firebase.google.com/go/v4"
	fbauth "firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"

	"github.com/tiennm99/dleague/server/internal/store"
)

// Firebase is the production Verifier — wraps the Firebase Admin SDK.
//
// Internal public-key cache lives inside the SDK client and survives across
// calls. First Verify after process start incurs one HTTPS round-trip.
type Firebase struct {
	client *fbauth.Client
}

// NewFirebase constructs the verifier from the JSON service-account creds
// already validated by config.Load.
func NewFirebase(ctx context.Context, credentialsJSON, projectID string) (*Firebase, error) {
	if credentialsJSON == "" || projectID == "" {
		return nil, fmt.Errorf("auth: credentialsJSON and projectID required")
	}
	app, err := firebase.NewApp(ctx,
		&firebase.Config{ProjectID: projectID},
		option.WithCredentialsJSON([]byte(credentialsJSON)),
	)
	if err != nil {
		return nil, fmt.Errorf("auth: firebase app: %w", err)
	}
	c, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("auth: firebase auth client: %w", err)
	}
	return &Firebase{client: c}, nil
}

// Verify validates the ID token and pulls out the claims dleague cares about.
func (f *Firebase) Verify(ctx context.Context, idToken string) (store.AuthClaims, error) {
	if f == nil || f.client == nil {
		return store.AuthClaims{}, fmt.Errorf("auth: verifier closed")
	}
	tok, err := f.client.VerifyIDToken(ctx, idToken)
	if err != nil {
		return store.AuthClaims{}, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	return claimsFrom(tok), nil
}

func claimsFrom(tok *fbauth.Token) store.AuthClaims {
	c := store.AuthClaims{UID: tok.UID}
	if v, ok := tok.Claims["email"].(string); ok {
		c.Email = v
	}
	if v, ok := tok.Claims["name"].(string); ok {
		c.DisplayName = v
	}
	if fb, ok := tok.Claims["firebase"].(map[string]any); ok {
		if v, ok := fb["sign_in_provider"].(string); ok {
			c.Provider = v
		}
	}
	return c
}
