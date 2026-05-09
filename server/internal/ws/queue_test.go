package ws

import (
	"sync"
	"testing"
)

// newTestConn returns a minimal Conn with a userID suitable for queue tests.
func newTestConnUID(uid string) *Conn {
	return &Conn{
		userID: uid,
		send:   make(chan []byte, sendBufSize),
	}
}

func TestQueue_PairOnSecondPush(t *testing.T) {
	t.Parallel()
	q := NewQueue()
	a := newTestConnUID("user-a")
	b := newTestConnUID("user-b")

	// Only one player: push a; no pair available yet.
	q.Push("wordle", a)
	if _, _, ok := q.PopPair("wordle"); ok {
		t.Fatal("PopPair should return false with only one player")
	}
	// PopPair with <2 items must not remove anything, so a is still in the queue.
	// Push b: now two players are present.
	q.Push("wordle", b)
	gotA, gotB, ok := q.PopPair("wordle")
	if !ok {
		t.Fatal("PopPair should succeed with two players")
	}
	if gotA != a || gotB != b {
		t.Fatalf("FIFO order: want (user-a,user-b), got (%v,%v)", gotA.userID, gotB.userID)
	}
}

func TestQueue_FIFOOrder(t *testing.T) {
	t.Parallel()
	q := NewQueue()
	conns := make([]*Conn, 6)
	for i := range conns {
		conns[i] = newTestConnUID("user-" + string(rune('a'+i)))
		q.Push("wordle", conns[i])
	}

	for round := range 3 {
		a, b, ok := q.PopPair("wordle")
		if !ok {
			t.Fatalf("round %d: expected pair", round)
		}
		if a != conns[round*2] || b != conns[round*2+1] {
			t.Fatalf("round %d: FIFO violated: got %s,%s want %s,%s",
				round, a.userID, b.userID,
				conns[round*2].userID, conns[round*2+1].userID)
		}
	}
}

func TestQueue_Remove(t *testing.T) {
	t.Parallel()
	q := NewQueue()
	a := newTestConnUID("user-a")
	b := newTestConnUID("user-b")

	q.Push("wordle", a)
	q.Push("wordle", b)
	q.Remove(a)

	// After removing a, only b remains — no pair.
	if _, _, ok := q.PopPair("wordle"); ok {
		t.Fatal("PopPair should return false after removing one player")
	}
}

func TestQueue_ConcurrentPush_Race(t *testing.T) {
	t.Parallel()
	q := NewQueue()
	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		go func(idx int) {
			defer wg.Done()
			c := newTestConnUID("user-" + string(rune('a'+idx%26)))
			q.Push("wordle", c)
		}(i)
	}
	wg.Wait()

	// Drain all pairs without panicking (race detector validates no data races).
	for {
		_, _, ok := q.PopPair("wordle")
		if !ok {
			break
		}
	}
}

func TestQueue_EmptyPopPair(t *testing.T) {
	t.Parallel()
	q := NewQueue()
	if _, _, ok := q.PopPair("wordle"); ok {
		t.Fatal("PopPair on empty queue should return false")
	}
}
