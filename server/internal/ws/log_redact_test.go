package ws

import (
	"regexp"
	"testing"
)

// TestRedactUID_Determinism verifies that the same UID produces the same redacted output
// within a single process (salt is per-process).
func TestRedactUID_Determinism(t *testing.T) {
	uid := "test-user-123"
	first := RedactUID(uid)
	second := RedactUID(uid)
	if first != second {
		t.Fatalf("RedactUID not deterministic: %q != %q", first, second)
	}
}

// TestRedactUID_HexFormat verifies the output matches the expected pattern: u_ + 8 hex digits.
func TestRedactUID_HexFormat(t *testing.T) {
	tests := []struct {
		name    string
		uid     string
		pattern string
	}{
		{"non-empty uid", "user-123", "^u_[0-9a-f]{8}$"},
		{"empty uid", "", "^u_anon$"},
		{"long uid", "very-long-user-identifier-string-that-is-quite-long", "^u_[0-9a-f]{8}$"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RedactUID(tt.uid)
			matched, err := regexp.MatchString(tt.pattern, result)
			if err != nil {
				t.Fatalf("regex error: %v", err)
			}
			if !matched {
				t.Fatalf("RedactUID(%q) = %q, does not match pattern %q", tt.uid, result, tt.pattern)
			}
		})
	}
}

// TestRedactUID_DifferentInputs verifies that different UIDs produce different redacted outputs.
func TestRedactUID_DifferentInputs(t *testing.T) {
	uid1 := "user-one"
	uid2 := "user-two"
	redacted1 := RedactUID(uid1)
	redacted2 := RedactUID(uid2)
	if redacted1 == redacted2 {
		t.Fatalf("different UIDs produced same redacted output: %q vs %q", redacted1, redacted2)
	}
}

// TestTruncateToken_LongToken verifies a token longer than 8 chars is truncated.
func TestTruncateToken_LongToken(t *testing.T) {
	token := "1234567890abcdefghijklmnop"
	result := TruncateToken(token)
	expected := "12345678…"
	if result != expected {
		t.Fatalf("TruncateToken(%q) = %q, want %q", token, result, expected)
	}
}

// TestTruncateToken_ShortToken verifies tokens ≤8 chars return "<short>".
func TestTruncateToken_ShortToken(t *testing.T) {
	tests := []struct {
		token string
		desc  string
	}{
		{"", "empty"},
		{"a", "1 char"},
		{"12345678", "exactly 8 chars"},
		{"short", "5 chars"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result := TruncateToken(tt.token)
			expected := "<short>"
			if result != expected {
				t.Fatalf("TruncateToken(%q) = %q, want %q", tt.token, result, expected)
			}
		})
	}
}
