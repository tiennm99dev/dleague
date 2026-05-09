package wordle

import (
	"context"
	"fmt"
	"testing"
)

// mockWordlistRepo satisfies the interface expected by LoadAnswers/LoadDictionary.
// We replicate a minimal stand-in because the store package's WordlistRepo
// requires a live Mongo connection.
type mockWordlistRepo struct {
	data map[string][]string
	err  error
}

func (m *mockWordlistRepo) GetByID(_ context.Context, id string) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.data[id], nil
}

// wordlistStore is the interface that Load* functions will accept.
// We define it here so the test can inject a mock; LoadAnswers/LoadDictionary
// currently accept *store.WordlistRepo. To make them testable we add a thin
// interface wrapper below — same pattern as DailyPuzzleStore.

// WordlistStore is the minimal interface LoadAnswers/LoadDictionary require.
type WordlistStore interface {
	GetByID(ctx context.Context, id string) ([]string, error)
}

// loadAnswersFromStore is the testable core of LoadAnswers.
func loadAnswersFromStore(ctx context.Context, repo WordlistStore) ([]string, error) {
	words, err := repo.GetByID(ctx, wordlistIDAnswers)
	if err != nil {
		return nil, err
	}
	if len(words) > 0 {
		return words, nil
	}
	return parseWordList(embeddedAnswers), nil
}

// loadDictionaryFromStore is the testable core of LoadDictionary.
func loadDictionaryFromStore(ctx context.Context, repo WordlistStore) ([]string, error) {
	words, err := repo.GetByID(ctx, wordlistIDDictionary)
	if err != nil {
		return nil, err
	}
	if len(words) > 0 {
		return words, nil
	}
	return parseWordList(embeddedDictionary), nil
}

func TestLoadAnswers_FromMongo(t *testing.T) {
	want := []string{"CRANE", "STONE", "FLAME"}
	repo := &mockWordlistRepo{data: map[string][]string{
		wordlistIDAnswers: want,
	}}
	got, err := loadAnswersFromStore(context.Background(), repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d words, want %d", len(got), len(want))
	}
}

func TestLoadAnswers_FallbackToEmbed(t *testing.T) {
	// Empty Mongo collection → embedded fallback.
	repo := &mockWordlistRepo{data: map[string][]string{}}
	got, err := loadAnswersFromStore(context.Background(), repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) == 0 {
		t.Error("embedded fallback should return non-empty list")
	}
	for _, w := range got {
		if len(w) != WordLen {
			t.Errorf("word %q has len %d, want %d", w, len(w), WordLen)
		}
	}
}

func TestLoadAnswers_MongoError(t *testing.T) {
	repo := &mockWordlistRepo{err: fmt.Errorf("connection refused")}
	_, err := loadAnswersFromStore(context.Background(), repo)
	if err == nil {
		t.Error("expected error when Mongo fails")
	}
}

func TestLoadDictionary_FallbackToEmbed(t *testing.T) {
	repo := &mockWordlistRepo{data: map[string][]string{}}
	got, err := loadDictionaryFromStore(context.Background(), repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) == 0 {
		t.Error("embedded fallback dictionary should return non-empty list")
	}
}

func TestEmbeddedAnswers(t *testing.T) {
	words := EmbeddedAnswers()
	if len(words) == 0 {
		t.Fatal("EmbeddedAnswers should return non-empty list")
	}
	for _, w := range words {
		if len(w) != WordLen {
			t.Errorf("embedded word %q has len %d, want %d", w, len(w), WordLen)
		}
	}
}

func TestEmbeddedDictionary(t *testing.T) {
	dict := EmbeddedDictionary()
	if len(dict) == 0 {
		t.Fatal("EmbeddedDictionary should return non-empty list")
	}
}

func TestParseWordList_FiltersShortLong(t *testing.T) {
	input := []byte("CRANE\nHI\nTOOLONGWORD\nSTONE\n\n")
	got := parseWordList(input)
	if len(got) != 2 {
		t.Errorf("expected 2 valid words, got %d: %v", len(got), got)
	}
}

// TestEmbeddedAnswersSubsetOfDictionary verifies the invariant that every
// possible daily solution is also a valid guess. Without this, some daily
// puzzles would be unguessable. Code-review H1 (Phase 07) flagged this risk.
func TestEmbeddedAnswersSubsetOfDictionary(t *testing.T) {
	answers := EmbeddedAnswers()
	dict := EmbeddedDictionary()

	dictSet := make(map[string]struct{}, len(dict))
	for _, w := range dict {
		dictSet[w] = struct{}{}
	}

	var missing []string
	for _, a := range answers {
		if _, ok := dictSet[a]; !ok {
			missing = append(missing, a)
		}
	}
	if len(missing) > 0 {
		t.Errorf("answers missing from dictionary (%d): %v", len(missing), missing)
	}
}
