package redis_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	rstore "github.com/tiennm99/dleague/server/internal/store/redis"
)

// startRedis spins up an in-process Redis fake and returns a wired Client.
func startRedis(t *testing.T) (*miniredis.Miniredis, *rstore.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	c := rstore.NewFromClient(goredis.NewClient(&goredis.Options{Addr: mr.Addr()}))
	t.Cleanup(func() { _ = c.Close() })
	return mr, c
}

func TestSubmitScoreGTAndTopN(t *testing.T) {
	_, c := startRedis(t)
	ctx := context.Background()
	const board = "lb:daily:2026-05-05"

	mustSubmit := func(uid string, sc int64) {
		t.Helper()
		if err := c.SubmitScore(ctx, board, uid, sc); err != nil {
			t.Fatalf("SubmitScore(%s,%d): %v", uid, sc, err)
		}
	}
	mustSubmit("u1", 100)
	mustSubmit("u1", 50)  // lower — must NOT replace
	mustSubmit("u1", 150) // higher — must replace
	mustSubmit("u2", 200)
	mustSubmit("u3", 175)

	top, err := c.TopN(ctx, board, 10)
	if err != nil {
		t.Fatalf("TopN: %v", err)
	}
	if len(top) != 3 {
		t.Fatalf("len = %d, want 3 (got: %+v)", len(top), top)
	}
	want := []struct {
		uid string
		sc  int64
	}{{"u2", 200}, {"u3", 175}, {"u1", 150}}
	for i, w := range want {
		if top[i].UID != w.uid || top[i].Score != w.sc {
			t.Errorf("top[%d] = %+v, want {uid:%s score:%d}", i, top[i], w.uid, w.sc)
		}
	}
}

func TestPresenceTTL(t *testing.T) {
	mr, c := startRedis(t)
	ctx := context.Background()

	if err := c.MarkOnline(ctx, "u1", 60*time.Second); err != nil {
		t.Fatal(err)
	}
	on, err := c.IsOnline(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if !on {
		t.Fatal("expected online")
	}
	mr.FastForward(61 * time.Second)
	on, err = c.IsOnline(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if on {
		t.Fatal("expected offline after TTL")
	}
}

func TestCacheGetSet(t *testing.T) {
	mr, c := startRedis(t)
	ctx := context.Background()

	if err := c.CacheSet(ctx, "k", []byte("hello"), 10*time.Second); err != nil {
		t.Fatal(err)
	}
	v, ok, err := c.CacheGet(ctx, "k")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || string(v) != "hello" {
		t.Fatalf("CacheGet = (%q, %v)", v, ok)
	}

	mr.FastForward(11 * time.Second)
	_, ok, err = c.CacheGet(ctx, "k")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected expiry")
	}

	// miss returns ok=false, err=nil
	_, ok, err = c.CacheGet(ctx, "missing")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("missing key should miss")
	}
}
