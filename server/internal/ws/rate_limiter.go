package ws

import (
	"sync"
	"time"
)

const (
	rateLimitTokensMax  = 10.0
	rateLimitRefillRate = 10.0 // tokens per second
)

// RateLimiter is a per-connection token-bucket rate limiter.
// 10 token burst capacity; refills at 10 tokens/sec.
// All methods are goroutine-safe.
type RateLimiter struct {
	mu     sync.Mutex
	tokens float64
	last   time.Time
}

// NewRateLimiter returns a full RateLimiter (10 initial tokens).
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		tokens: rateLimitTokensMax,
		last:   time.Now(),
	}
}

// Allow returns true and consumes one token if a token is available.
// Returns false (without consuming) when the bucket is empty.
// Refills tokens based on elapsed time since last call (lazy refill).
func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.last).Seconds()
	r.last = now

	// Lazy refill: add tokens proportional to elapsed time, capped at max.
	r.tokens += elapsed * rateLimitRefillRate
	if r.tokens > rateLimitTokensMax {
		r.tokens = rateLimitTokensMax
	}

	if r.tokens < 1.0 {
		return false
	}
	r.tokens--
	return true
}
