package ws

import (
	"sync"
	"testing"
	"time"
)

// TestUIDLimiter_AllowFresh verifies that a new UID gets full burst capacity.
func TestUIDLimiter_AllowFresh(t *testing.T) {
	limiter := NewUIDLimiter(10, 3, time.Second) // 10 tokens/sec, 3 burst, 1s TTL
	uid := "test-user"

	// First 3 calls should succeed (burst capacity).
	for i := 0; i < 3; i++ {
		if !limiter.Allow(uid) {
			t.Fatalf("Allow #%d failed, expected success within burst", i+1)
		}
	}

	// 4th call should fail (bucket exhausted).
	if limiter.Allow(uid) {
		t.Fatalf("Allow #4 succeeded, expected rate-limit after burst exhausted")
	}
}

// TestUIDLimiter_PerUIDIsolation verifies UIDs A and B maintain separate buckets.
func TestUIDLimiter_PerUIDIsolation(t *testing.T) {
	limiter := NewUIDLimiter(10, 2, time.Second) // 2-token burst
	uidA := "user-a"
	uidB := "user-b"

	// Exhaust UID A's burst.
	if !limiter.Allow(uidA) || !limiter.Allow(uidA) {
		t.Fatal("failed to exhaust UID A's burst")
	}
	if limiter.Allow(uidA) {
		t.Fatal("UID A should be rate-limited after burst")
	}

	// UID B should still have full burst.
	if !limiter.Allow(uidB) || !limiter.Allow(uidB) {
		t.Fatal("UID B should have full burst despite A being limited")
	}
	if limiter.Allow(uidB) {
		t.Fatal("UID B should be rate-limited after its own burst")
	}
}

// TestUIDLimiter_EvictIdle verifies idle buckets are removed.
func TestUIDLimiter_EvictIdle(t *testing.T) {
	ttl := 50 * time.Millisecond
	limiter := NewUIDLimiter(10, 1, ttl)
	uid := "test-user"

	// Create a bucket for this UID.
	limiter.Allow(uid)

	// Verify bucket exists by checking Allow returns false (burst exhausted).
	if limiter.Allow(uid) {
		t.Fatal("expected bucket to be exhausted")
	}

	// Wait for TTL to expire.
	time.Sleep(ttl + 10*time.Millisecond)

	// Call EvictIdle.
	limiter.EvictIdle()

	// Bucket should now be removed — next Allow should start fresh.
	if !limiter.Allow(uid) {
		t.Fatal("after eviction, expected fresh bucket with available tokens")
	}
}

// TestUIDLimiter_RaceSafety verifies concurrent Allow calls don't race.
func TestUIDLimiter_RaceSafety(t *testing.T) {
	limiter := NewUIDLimiter(100, 1000, time.Second) // high burst to reduce contention-induced failures
	uid := "concurrent-test"

	var wg sync.WaitGroup
	allowed := 0
	var mu sync.Mutex

	// Spawn 10 goroutines, each calling Allow 100 times.
	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				if limiter.Allow(uid) {
					mu.Lock()
					allowed++
					mu.Unlock()
				}
			}
		}()
	}

	wg.Wait()

	// With burst=1000 and 1000 total calls, we should see most allowed.
	// The exact number depends on timing/refill, but all <= 1000 total.
	if allowed > 1000 {
		t.Fatalf("allowed count %d exceeds burst capacity", allowed)
	}
	if allowed < 100 {
		t.Fatalf("allowed count %d is suspiciously low (possible race)", allowed)
	}
}

// TestUIDLimiter_TokenRefill verifies tokens refill over time.
func TestUIDLimiter_TokenRefill(t *testing.T) {
	// 10 tokens/sec refill, 1 burst
	limiter := NewUIDLimiter(10, 1, time.Second)
	uid := "test-refill"

	// Exhaust initial burst.
	if !limiter.Allow(uid) {
		t.Fatal("failed to consume initial token")
	}

	// Immediately try again — should fail (bucket empty).
	if limiter.Allow(uid) {
		t.Fatal("expected refill delay, but Allow succeeded immediately")
	}

	// Wait for 1 token to refill (at 10 tokens/sec = 100ms per token).
	time.Sleep(150 * time.Millisecond)

	// Should now have a token.
	if !limiter.Allow(uid) {
		t.Fatal("expected token after refill period")
	}
}
