package ws

import (
	"context"
	"fmt"
	"time"

	"firebase.google.com/go/v4/auth"
	"google.golang.org/protobuf/proto"

	dleaguev1 "github.com/tiennm99/dleague/shared/pb/dleague/v1"
)

// handleAuthRefresh processes a MESSAGE_TYPE_AUTH_REFRESH envelope.
//
// It re-verifies the supplied Firebase ID token, updates c.userID,
// c.isAnonymous, and c.tokenExpiresAt on the connection, then returns an
// AuthRefreshAck envelope. On verification failure it enqueues an ERROR{401}
// and cancels the read context (closes the connection).
func handleAuthRefresh(ctx context.Context, c *Conn, env *dleaguev1.Envelope) (*dleaguev1.Envelope, error) {
	if c.hub.verifier == nil {
		return errorEnvelope(env.GetRequestId(), 500, "auth not configured"), nil
	}

	var msg dleaguev1.AuthRefresh
	if err := proto.Unmarshal(env.GetPayload(), &msg); err != nil {
		return errorEnvelope(env.GetRequestId(), 400, "invalid auth_refresh payload"), nil
	}

	token, err := c.hub.verifier.VerifyIDToken(ctx, msg.GetIdToken())
	if err != nil {
		// Auth failure: send 401 and close the connection.
		c.enqueue(errorEnvelope(env.GetRequestId(), 401, "auth refresh failed"))
		c.cancelRead()
		return nil, nil
	}

	// AuthRefresh must NOT be used to switch identities mid-connection.
	// An attacker with a valid token for user B could otherwise pivot a
	// session originally authenticated as user A. Anon→authenticated upgrade
	// is a separate flow (open a fresh connection with the new token).
	// Read current userID under lock to avoid race with cross-goroutine readers.
	c.mu.RLock()
	currentUID := c.userID
	c.mu.RUnlock()
	if currentUID != "" && token.UID != currentUID {
		c.enqueue(errorEnvelope(env.GetRequestId(), 401, "auth refresh uid mismatch"))
		c.cancelRead()
		return nil, nil
	}

	// Update connection identity from refreshed token (lock covers all four fields).
	c.mu.Lock()
	c.userID = token.UID
	c.tokenExpiresAt = time.Unix(token.Expires, 0)
	c.isAnonymous = isAnonymousToken(token)
	c.isAdmin, _ = token.Claims["admin"].(bool)
	c.mu.Unlock()

	ack := &dleaguev1.AuthRefreshAck{ExpiresAtUnix: token.Expires}
	body, err := proto.Marshal(ack)
	if err != nil {
		return nil, fmt.Errorf("auth_refresh: marshal ack: %w", err)
	}

	return &dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_AUTH_REFRESH_ACK,
		RequestId: env.GetRequestId(),
		Payload:   body,
	}, nil
}

// isAnonymousToken returns true when the Firebase token's sign-in provider is
// "anonymous". Uses the SDK's typed accessor (auth.Token.Firebase.SignInProvider)
// rather than a manual type assertion on the raw claims map.
func isAnonymousToken(token *auth.Token) bool {
	if token == nil {
		return false
	}
	return token.Firebase.SignInProvider == "anonymous"
}
