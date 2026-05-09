package ws

import (
	"testing"
	"time"
)

func TestRateLimiter_Burst(t *testing.T) {
	t.Parallel()
	rl := NewRateLimiter()

	// First 10 calls must succeed (full bucket).
	for i := range 10 {
		if !rl.Allow() {
			t.Fatalf("call %d: expected Allow()=true within burst capacity", i+1)
		}
	}
	// 11th call must be denied.
	if rl.Allow() {
		t.Fatal("11th call: expected Allow()=false after burst exhausted")
	}
}

func TestRateLimiter_Refill(t *testing.T) {
	t.Parallel()
	rl := NewRateLimiter()

	// Drain the bucket.
	for range 10 {
		rl.Allow()
	}
	if rl.Allow() {
		t.Fatal("bucket should be empty before refill")
	}

	// Wait long enough for at least 1 token to refill (~100ms at 10/sec).
	time.Sleep(120 * time.Millisecond)
	if !rl.Allow() {
		t.Fatal("expected Allow()=true after 120ms refill")
	}
}

func TestRateLimiter_DenyPath(t *testing.T) {
	t.Parallel()
	rl := NewRateLimiter()

	// Drain.
	for range 10 {
		rl.Allow()
	}

	// Rapid burst of denials — must all return false without panicking.
	denials := 0
	for range 20 {
		if !rl.Allow() {
			denials++
		}
	}
	if denials == 0 {
		t.Fatal("expected at least some denials after bucket exhaustion")
	}
}

func TestRateLimiter_Concurrent(t *testing.T) {
	t.Parallel()
	rl := NewRateLimiter()

	const goroutines = 50
	results := make(chan bool, goroutines)
	for range goroutines {
		go func() { results <- rl.Allow() }()
	}

	allowed := 0
	for range goroutines {
		if <-results {
			allowed++
		}
	}
	// Only 10 tokens available; at most 10 goroutines should succeed.
	if allowed > 10 {
		t.Fatalf("concurrent: expected ≤10 allowed, got %d", allowed)
	}
}
