package ws

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"
	"nhooyr.io/websocket"

	"github.com/tiennm99/dleague/server/internal/store"
	dleaguev1 "github.com/tiennm99/dleague/shared/pb/dleague/v1"
)

// HandshakeTimeout caps the wait between WS-accept and the AUTH_REQUEST
// frame. 5s is the project default; bump via UpgradeOptions if a profile
// needs longer.
const HandshakeTimeout = 5 * time.Second

// CloseUnauthenticated is the WS close code sent when the handshake fails
// (timeout, malformed frame, invalid token). 4001 is in the
// application-private range.
const CloseUnauthenticated websocket.StatusCode = 4001

// TokenVerifier is the seam between this package and `internal/auth`. The
// production wiring passes `*auth.Gate`; tests pass a fake.
type TokenVerifier interface {
	Verify(ctx context.Context, idToken string) (store.AuthClaims, error)
}

// Handshake errors. They are not exported as separate sentinels because the
// remote side gets a typed wire response; logs see a wrapped error.
var (
	errHandshakeTimeout    = errors.New("handshake: timeout")
	errHandshakeMalformed  = errors.New("handshake: malformed first frame")
	errHandshakeWrongType  = errors.New("handshake: first frame not AUTH_REQUEST")
	errHandshakeNoVerifier = errors.New("handshake: no verifier configured")
)

// performHandshake reads one frame, expects AUTH_REQUEST, verifies the token,
// and writes AUTH_RESPONSE. On success it returns the claims; the caller
// stamps the conn's UID. On failure it sends an AUTH_RESPONSE{ok:false} when
// possible (best-effort), then closes the conn with CloseUnauthenticated.
func performHandshake(ctx context.Context, c *websocket.Conn, v TokenVerifier) (store.AuthClaims, error) {
	if v == nil {
		return store.AuthClaims{}, errHandshakeNoVerifier
	}

	readCtx, cancel := context.WithTimeout(ctx, HandshakeTimeout)
	defer cancel()

	mt, data, err := c.Read(readCtx)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return store.AuthClaims{}, errHandshakeTimeout
		}
		return store.AuthClaims{}, fmt.Errorf("handshake read: %w", err)
	}
	if mt != websocket.MessageBinary {
		return store.AuthClaims{}, errHandshakeMalformed
	}

	var env dleaguev1.Envelope
	if err := proto.Unmarshal(data, &env); err != nil {
		return store.AuthClaims{}, errHandshakeMalformed
	}
	if env.GetType() != dleaguev1.MessageType_MESSAGE_TYPE_AUTH_REQUEST {
		return store.AuthClaims{}, errHandshakeWrongType
	}

	var req dleaguev1.AuthRequest
	if err := proto.Unmarshal(env.GetPayload(), &req); err != nil {
		return store.AuthClaims{}, errHandshakeMalformed
	}

	claims, verifyErr := v.Verify(readCtx, req.GetIdToken())

	resp := buildAuthResponse(env.GetRequestId(), claims, verifyErr)
	if writeErr := writeEnvelope(ctx, c, resp); writeErr != nil {
		// Best-effort — if we can't write the rejection, the client gets a
		// raw close anyway.
		_ = writeErr
	}
	if verifyErr != nil {
		return store.AuthClaims{}, fmt.Errorf("handshake verify: %w", verifyErr)
	}
	return claims, nil
}

func buildAuthResponse(requestID string, claims store.AuthClaims, verifyErr error) *dleaguev1.Envelope {
	body := &dleaguev1.AuthResponse{Ok: verifyErr == nil, Uid: claims.UID}
	if verifyErr != nil {
		body.Error = "invalid_token"
	}
	payload, _ := proto.Marshal(body)
	return &dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_AUTH_RESPONSE,
		RequestId: requestID,
		Payload:   payload,
	}
}

func writeEnvelope(ctx context.Context, c *websocket.Conn, env *dleaguev1.Envelope) error {
	out, err := proto.Marshal(env)
	if err != nil {
		return err
	}
	writeCtx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	return c.Write(writeCtx, websocket.MessageBinary, out)
}
