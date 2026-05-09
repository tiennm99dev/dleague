package ws

import dleaguev1 "github.com/tiennm99/dleague/shared/pb/dleague/v1"

// requiresAuth reports whether a message of the given type must only be
// dispatched when the connection has an authenticated user (c.userID != "").
//
// The allowlist approach is intentionally conservative: any message type not
// explicitly listed here requires authentication. Messages in the deny set are
// safe to process unauthenticated (protocol-level or auth-channel messages).
func requiresAuth(t dleaguev1.MessageType) bool {
	switch t {
	case dleaguev1.MessageType_MESSAGE_TYPE_UNSPECIFIED,
		dleaguev1.MessageType_MESSAGE_TYPE_PING,
		dleaguev1.MessageType_MESSAGE_TYPE_PONG,
		dleaguev1.MessageType_MESSAGE_TYPE_AUTH_REFRESH,
		dleaguev1.MessageType_MESSAGE_TYPE_AUTH_REFRESH_ACK,
		dleaguev1.MessageType_MESSAGE_TYPE_ERROR:
		return false
	default:
		return true
	}
}
