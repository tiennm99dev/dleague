package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/tiennm99/dleague/server/internal/api"
	"github.com/tiennm99/dleague/server/internal/store"
	"github.com/tiennm99/dleague/server/internal/store/memstore"
)

type fakeVerifier struct{ uid string }

func (f *fakeVerifier) Verify(_ context.Context, _ string) (store.AuthClaims, error) {
	return store.AuthClaims{UID: f.uid, Provider: "password"}, nil
}

// setup wires a chi router with the api mounted, a memstore, and a verifier
// that grants every request the configured uid. Returns the server + store
// so tests can inspect side-effects.
func setup(t *testing.T, uid string) (*httptest.Server, *memstore.Store) {
	t.Helper()
	mem := memstore.New()
	t.Cleanup(func() { _ = mem.Close() })

	r := chi.NewRouter()
	api.Mount(r, mem, &fakeVerifier{uid: uid}, mem)

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv, mem
}

func todayUTC() string { return time.Now().UTC().Format("2006-01-02") }

func TestPuzzleGetReturnsHintNotSolution(t *testing.T) {
	srv, mem := setup(t, "u1")
	if err := mem.PutPuzzle(context.Background(), store.Puzzle{
		Date: todayUTC(), Word: "league", Hint: "noun", Difficulty: 3,
	}); err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(srv.URL + "/api/v1/puzzles/today")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if _, leaked := body["word"]; leaked {
		t.Error("response leaked the puzzle word")
	}
	if body["hint"] != "noun" {
		t.Errorf("hint = %v", body["hint"])
	}
	if int(body["length"].(float64)) != 6 {
		t.Errorf("length = %v", body["length"])
	}
}

func TestPuzzleGetUnknownDate404(t *testing.T) {
	srv, _ := setup(t, "u1")
	resp, err := http.Get(srv.URL + "/api/v1/puzzles/2026-05-05")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestPuzzleGetMalformedDate400(t *testing.T) {
	srv, _ := setup(t, "u1")
	resp, err := http.Get(srv.URL + "/api/v1/puzzles/not-a-date")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestSubmitAttemptScoresAndUpdatesLeaderboards(t *testing.T) {
	srv, mem := setup(t, "u1")
	date := todayUTC()
	if err := mem.PutPuzzle(context.Background(), store.Puzzle{Date: date, Word: "league"}); err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(map[string]any{
		"date":    date,
		"guesses": []string{"hello", "league"}, // solves on 2nd guess
	})
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/attempts", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer fake")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		buf, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d body = %s", resp.StatusCode, buf)
	}
	var ar struct {
		Score int64
		Won   bool
	}
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		t.Fatal(err)
	}
	if !ar.Won {
		t.Errorf("Won = false, want true")
	}
	if ar.Score != 85 { // 100 - 1*15
		t.Errorf("Score = %d, want 85", ar.Score)
	}

	// Persisted attempt is recoverable.
	got, err := mem.GetAttempt(context.Background(), "u1", date)
	if err != nil {
		t.Fatalf("GetAttempt: %v", err)
	}
	if got.Score != 85 || !got.Won {
		t.Errorf("attempt persisted incorrectly: %+v", got)
	}

	// Daily and global leaderboards both updated.
	for _, board := range []string{"lb:daily:" + date, "lb:global:alltime"} {
		top, err := mem.TopN(context.Background(), board, 5)
		if err != nil {
			t.Fatal(err)
		}
		if len(top) != 1 || top[0].UID != "u1" || top[0].Score != 85 {
			t.Errorf("board %s = %+v", board, top)
		}
	}
}

func TestSubmitAttemptResubmitKeepsHighScore(t *testing.T) {
	srv, mem := setup(t, "u1")
	date := todayUTC()
	if err := mem.PutPuzzle(context.Background(), store.Puzzle{Date: date, Word: "league"}); err != nil {
		t.Fatal(err)
	}

	post := func(guesses []string) int64 {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"date": date, "guesses": guesses})
		req, _ := http.NewRequest("POST", srv.URL+"/api/v1/attempts", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer fake")
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var ar struct{ Score int64 }
		if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
			t.Fatal(err)
		}
		return ar.Score
	}

	// First: 4 wrong + 1 right → 100 - 4*15 = 40.
	first := post([]string{"a", "b", "c", "d", "league"})
	if first != 40 {
		t.Errorf("first score = %d, want 40", first)
	}

	// Replay with a worse run — leaderboard must keep the higher score.
	second := post([]string{"a", "b", "c", "d", "e", "league"})
	if second != 25 { // 100 - 5*15
		t.Errorf("second score = %d, want 25", second)
	}

	top, err := mem.TopN(context.Background(), "lb:daily:"+date, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(top) != 1 || top[0].Score != 40 {
		t.Errorf("leaderboard regressed: %+v", top)
	}
}

func TestSubmitAttemptRejectsFutureDate(t *testing.T) {
	srv, mem := setup(t, "u1")
	future := time.Now().UTC().AddDate(0, 0, 7).Format("2006-01-02")
	if err := mem.PutPuzzle(context.Background(), store.Puzzle{Date: future, Word: "x"}); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"date": future, "guesses": []string{"x"}})
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/attempts", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer fake")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestSubmitAttemptRequiresAuth(t *testing.T) {
	srv, mem := setup(t, "u1")
	date := todayUTC()
	_ = mem.PutPuzzle(context.Background(), store.Puzzle{Date: date, Word: "x"})

	body, _ := json.Marshal(map[string]any{"date": date, "guesses": []string{"x"}})
	resp, err := http.Post(srv.URL+"/api/v1/attempts", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestGetAttemptMine(t *testing.T) {
	srv, mem := setup(t, "u1")
	date := todayUTC()
	_ = mem.UpsertAttempt(context.Background(), store.Attempt{
		UID: "u1", PuzzleDate: date, Guesses: []string{"a", "b"}, InProgress: true,
	})

	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/attempts/me/"+date, nil)
	req.Header.Set("Authorization", "Bearer fake")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var got store.Attempt
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.UID != "u1" || len(got.Guesses) != 2 {
		t.Errorf("got = %+v", got)
	}
}

func TestLeaderboardsScopeRouting(t *testing.T) {
	srv, mem := setup(t, "u1")
	ctx := context.Background()
	date := todayUTC()
	_ = mem.SubmitScore(ctx, "lb:global:alltime", "ualice", 500)
	_ = mem.SubmitScore(ctx, "lb:global:alltime", "ubob", 700)
	_ = mem.SubmitScore(ctx, "lb:daily:"+date, "ucarol", 100)

	cases := []struct {
		path      string
		wantTop   string
		wantScore int64
	}{
		{"/api/v1/leaderboards/global", "ubob", 700},
		{"/api/v1/leaderboards/daily/" + date, "ucarol", 100},
	}
	for _, tc := range cases {
		resp, err := http.Get(srv.URL + tc.path)
		if err != nil {
			t.Fatal(err)
		}
		var body struct {
			Rows []store.Rank
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if len(body.Rows) == 0 || body.Rows[0].UID != tc.wantTop || body.Rows[0].Score != tc.wantScore {
			t.Errorf("%s top = %+v", tc.path, body.Rows)
		}
	}
}

func TestScoringPureFunctionEdgeCases(t *testing.T) {
	cases := []struct {
		name     string
		solution string
		guesses  []string
		want     int64
		won      bool
	}{
		{"first guess wins", "league", []string{"league"}, 100, true},
		{"second guess wins", "league", []string{"x", "league"}, 85, true},
		{"case insensitive", "league", []string{"LEAGUE"}, 100, true},
		{"trims whitespace", "league", []string{"  league  "}, 100, true},
		{"loss", "league", []string{"a", "b", "c", "d", "e", "f"}, 0, false},
		{"empty guesses", "league", nil, 0, false},
		{"empty solution", "", []string{"x"}, 0, false},
		{"7th guess does not count as win", "league", []string{"a", "b", "c", "d", "e", "f", "league"}, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, w := api.Score(tc.solution, tc.guesses)
			if s != tc.want || w != tc.won {
				t.Errorf("score=%d won=%v, want %d/%v", s, w, tc.want, tc.won)
			}
		})
	}
}
