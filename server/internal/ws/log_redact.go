package ws

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

var logSalt []byte

func init() {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// best-effort fallback — redaction still hides UID even with empty salt
		b = []byte("dleague-fallback")
	}
	logSalt = b
}

// RedactUID returns "u_" + 8 hex chars of HMAC-SHA256(salt, uid).
// Per-process salt: stable within run, rotates on restart.
func RedactUID(uid string) string {
	if uid == "" {
		return "u_anon"
	}
	m := hmac.New(sha256.New, logSalt)
	m.Write([]byte(uid))
	sum := m.Sum(nil)
	return "u_" + hex.EncodeToString(sum)[:8]
}

// TruncateToken returns the first 8 chars of token + "…" — never log full bearer.
func TruncateToken(token string) string {
	if len(token) <= 8 {
		return "<short>"
	}
	return token[:8] + "…"
}
