package ws

import (
	"context"
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

// allow is the internal single-token consume used by both RateLimiter and UIDLimiter.
// Caller must hold the lock.
func (r *RateLimiter) allow() bool {
	now := time.Now()
	elapsed := now.Sub(r.last).Seconds()
	r.last = now
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

// Allow returns true and consumes one token if a token is available.
// Returns false (without consuming) when the bucket is empty.
func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.allow()
}

// ── Per-UID rate limiter ──────────────────────────────────────────────────────

// uidBucket is a token bucket entry used by UIDLimiter.
type uidBucket struct {
	tokens float64
	last   time.Time
}

func (b *uidBucket) allow(rate, burst float64) bool {
	now := time.Now()
	elapsed := now.Sub(b.last).Seconds()
	b.last = now
	b.tokens += elapsed * rate
	if b.tokens > burst {
		b.tokens = burst
	}
	if b.tokens < 1.0 {
		return false
	}
	b.tokens--
	return true
}

// UIDLimiter is a per-UID token-bucket rate limiter with TTL-based eviction.
// Goroutine-safe; designed for defence-in-depth above the per-conn limiter.
type UIDLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*uidBucket
	lastSeen map[string]time.Time
	rate     float64
	burst    float64
	ttl      time.Duration
}

// NewUIDLimiter creates a UIDLimiter with the given token-refill rate (tokens/sec),
// burst capacity, and idle TTL for eviction.
func NewUIDLimiter(rate, burst float64, ttl time.Duration) *UIDLimiter {
	return &UIDLimiter{
		buckets:  make(map[string]*uidBucket),
		lastSeen: make(map[string]time.Time),
		rate:     rate,
		burst:    burst,
		ttl:      ttl,
	}
}

// Allow returns true if uid has available tokens, false on rate-limit.
func (l *UIDLimiter) Allow(uid string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[uid]
	if !ok {
		b = &uidBucket{tokens: l.burst, last: time.Now()}
		l.buckets[uid] = b
	}
	l.lastSeen[uid] = time.Now()
	return b.allow(l.rate, l.burst)
}

// EvictIdle removes buckets that have been idle for longer than ttl.
func (l *UIDLimiter) EvictIdle() {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	for uid, t := range l.lastSeen {
		if now.Sub(t) > l.ttl {
			delete(l.buckets, uid)
			delete(l.lastSeen, uid)
		}
	}
}

// RunEvictor runs EvictIdle on the given interval until ctx is done.
func (l *UIDLimiter) RunEvictor(ctx context.Context, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			l.EvictIdle()
		}
	}
}
