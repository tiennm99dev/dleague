package ws

import (
	"sync"
	"testing"
	"time"
)

// TestAuthRefreshAccessorsRace verifies that concurrent updates to auth fields
// (from auth_refresh) do not race with reads (UserID, IsAnonymous, IsAdmin).
// This is a Phase 01 regression test for the lock added around these fields.
func TestAuthRefreshAccessorsRace(t *testing.T) {
	c := &Conn{
		userID:      "initial-uid",
		isAnonymous: false,
		isAdmin:     false,
		tokenExpiresAt: time.Now().Add(time.Hour),
	}

	var wg sync.WaitGroup
	done := make(chan struct{})

	// Goroutine A: repeatedly updates auth fields (simulating handleAuthRefresh updates).
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			select {
			case <-done:
				return
			default:
			}
			// Simulate what handleAuthRefresh does: update all four fields under lock.
			c.mu.Lock()
			c.userID = "uid-" + string(rune(i%256))
			c.isAnonymous = (i % 2) == 0
			c.isAdmin = (i % 3) == 0
			c.tokenExpiresAt = time.Now().Add(time.Hour)
			c.mu.Unlock()
		}
	}()

	// Goroutine B: repeatedly reads auth fields (via accessors).
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			select {
			case <-done:
				return
			default:
			}
			_ = c.UserID()
			_ = c.IsAnonymous()
			// Intentionally read tokenExpiresAt directly to test all paths.
			c.mu.RLock()
			_ = c.tokenExpiresAt
			c.mu.RUnlock()
		}
	}()

	// Goroutine C: also reads, to increase contention.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			select {
			case <-done:
				return
			default:
			}
			_ = c.UserID()
			_ = c.IsAnonymous()
		}
	}()

	// Let them run in parallel.
	time.Sleep(100 * time.Millisecond)
	close(done)
	wg.Wait()

	// If we got here without -race reporting a data race, test passes.
	// The mutex ensures all four fields are read/written atomically.
}

// TestConnAccessorsRead verifies basic accessor correctness after initialization.
func TestConnAccessorsRead(t *testing.T) {
	c := &Conn{
		userID:      "test-uid-123",
		isAnonymous: true,
		isAdmin:     false,
	}

	if got := c.UserID(); got != "test-uid-123" {
		t.Fatalf("UserID() = %q, want %q", got, "test-uid-123")
	}

	if got := c.IsAnonymous(); got != true {
		t.Fatalf("IsAnonymous() = %v, want true", got)
	}
}

// TestActiveMatchIDAccessor verifies atomic get/set of activeMatchID.
func TestActiveMatchIDAccessor(t *testing.T) {
	c := &Conn{}

	// Initial value should be empty.
	if got := c.getActiveMatchID(); got != "" {
		t.Fatalf("initial activeMatchID = %q, want empty", got)
	}

	// Set a value.
	c.setActiveMatchID("match-123")
	if got := c.getActiveMatchID(); got != "match-123" {
		t.Fatalf("after set, activeMatchID = %q, want %q", got, "match-123")
	}

	// Clear it.
	c.setActiveMatchID("")
	if got := c.getActiveMatchID(); got != "" {
		t.Fatalf("after clear, activeMatchID = %q, want empty", got)
	}
}

// TestActiveMatchIDRace verifies concurrent set/get on activeMatchID do not race.
func TestActiveMatchIDRace(t *testing.T) {
	c := &Conn{}
	var wg sync.WaitGroup
	done := make(chan struct{})

	// Goroutine A: repeatedly set activeMatchID.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			select {
			case <-done:
				return
			default:
			}
			c.setActiveMatchID("match-" + string(rune(i%256)))
		}
	}()

	// Goroutine B: repeatedly get activeMatchID.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			select {
			case <-done:
				return
			default:
			}
			_ = c.getActiveMatchID()
		}
	}()

	time.Sleep(100 * time.Millisecond)
	close(done)
	wg.Wait()
}
