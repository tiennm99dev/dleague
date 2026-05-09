package wordle

import (
	"context"
	"testing"
	"time"

	"github.com/tiennm99/dleague/server/internal/store"
)

// mockDailyRepo is an in-memory DailyPuzzleStore for unit tests.
// It satisfies the DailyPuzzleStore interface defined in daily.go.
type mockDailyRepo struct {
	data map[string]*store.DailyPuzzle
}

func newMockDailyRepo() *mockDailyRepo {
	return &mockDailyRepo{data: make(map[string]*store.DailyPuzzle)}
}

func (m *mockDailyRepo) GetByDate(_ context.Context, date string) (*store.DailyPuzzle, error) {
	p, ok := m.data[date]
	if !ok {
		return nil, nil
	}
	return p, nil
}

func (m *mockDailyRepo) Upsert(_ context.Context, p store.DailyPuzzle) error {
	m.data[p.ID] = &p
	return nil
}

var testAnswers = []string{
	"ABOUT", "ABOVE", "ABUSE", "ACTOR", "ACUTE",
	"ADMIT", "ADOPT", "ADULT", "AFTER", "AGAIN",
	"CRANE", "STONE", "FLAME", "BRAVE", "CHASE",
	"DANCE", "EARLY", "FEAST", "GIANT", "HEART",
}

func TestEnsureToday_CreatesDocument(t *testing.T) {
	repo := newMockDailyRepo()
	day := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)

	solution, err := EnsureToday(context.Background(), repo, testAnswers, day)
	if err != nil {
		t.Fatalf("EnsureToday: %v", err)
	}
	if solution == "" {
		t.Fatal("solution should not be empty")
	}

	doc, err := repo.GetByDate(context.Background(), "2026-05-09")
	if err != nil {
		t.Fatalf("GetByDate: %v", err)
	}
	if doc == nil {
		t.Fatal("document should have been created")
	}
	if doc.Solution != solution {
		t.Errorf("stored solution %q != returned solution %q", doc.Solution, solution)
	}
	if doc.GameID != "wordle" {
		t.Errorf("GameID = %q, want wordle", doc.GameID)
	}
	if doc.SolutionHash == "" {
		t.Error("SolutionHash should not be empty")
	}
}

func TestEnsureToday_SameDateReturnsSameSolution(t *testing.T) {
	repo := newMockDailyRepo()
	day1 := time.Date(2026, 5, 9, 14, 30, 0, 0, time.UTC)
	day2 := time.Date(2026, 5, 9, 23, 59, 0, 0, time.UTC)

	sol1, err := EnsureToday(context.Background(), repo, testAnswers, day1)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	sol2, err := EnsureToday(context.Background(), repo, testAnswers, day2)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if sol1 != sol2 {
		t.Errorf("same UTC date must yield same solution: %q vs %q", sol1, sol2)
	}
}

func TestEnsureToday_DifferentDatesLikelyDifferent(t *testing.T) {
	repo := newMockDailyRepo()
	day1 := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)

	sol1, _ := EnsureToday(context.Background(), repo, testAnswers, day1)
	sol2, _ := EnsureToday(context.Background(), repo, testAnswers, day2)

	if sol1 == sol2 {
		// With a small placeholder list collision is possible; log rather than fail.
		t.Logf("WARN: consecutive dates produced same solution (%q)", sol1)
	}
}

func TestEnsureToday_ExistingDocNotOverwritten(t *testing.T) {
	repo := newMockDailyRepo()
	day := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)

	_ = repo.Upsert(context.Background(), store.DailyPuzzle{
		ID:       "2026-05-09",
		GameID:   "wordle",
		Solution: "CRANE",
	})

	sol, err := EnsureToday(context.Background(), repo, testAnswers, day)
	if err != nil {
		t.Fatalf("EnsureToday with pre-seeded doc: %v", err)
	}
	if sol != "CRANE" {
		t.Errorf("should return existing solution, got %q", sol)
	}
}

func TestEnsureToday_EmptyAnswers(t *testing.T) {
	repo := newMockDailyRepo()
	day := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)
	_, err := EnsureToday(context.Background(), repo, nil, day)
	if err == nil {
		t.Error("expected error for empty answers list")
	}
}
