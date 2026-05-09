package wordle

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/tiennm99/dleague/server/internal/store"
)

// DailyPuzzleStore is the minimal interface required by EnsureToday.
// *store.DailyPuzzleRepo satisfies this interface; tests inject a mock.
type DailyPuzzleStore interface {
	GetByDate(ctx context.Context, date string) (*store.DailyPuzzle, error)
	Upsert(ctx context.Context, p store.DailyPuzzle) error
}

// EnsureToday guarantees a DailyPuzzle document exists for today's UTC date.
// If the document already exists its solution is returned unchanged (idempotent).
// If absent a deterministic word is chosen from answers and upserted.
//
// Seed algorithm (documented for audit / cheat investigation):
//
//	hash      = sha256(date + "wordle-v1")
//	seed      = int64( BigEndian.Uint64(hash[:8]) & 0x7FFF_FFFF_FFFF_FFFF )
//	word      = answers[ seed % len(answers) ]
//
// The solution is stored in Mongo but NEVER returned to clients until terminal.
func EnsureToday(ctx context.Context, repo DailyPuzzleStore, answers []string, now time.Time) (string, error) {
	return ensureTodayImpl(ctx, repo, answers, now)
}

// ensureTodayImpl is the shared implementation used by both EnsureToday and tests.
func ensureTodayImpl(ctx context.Context, repo DailyPuzzleStore, answers []string, now time.Time) (string, error) {
	if len(answers) == 0 {
		return "", fmt.Errorf("wordle: EnsureToday: empty answers list")
	}

	date := now.UTC().Format("2006-01-02")

	existing, err := repo.GetByDate(ctx, date)
	if err != nil {
		return "", fmt.Errorf("wordle: EnsureToday GetByDate: %w", err)
	}
	if existing != nil {
		return existing.Solution, nil
	}

	// Compute deterministic seed from date string.
	h := sha256.Sum256([]byte(date + "wordle-v1"))
	rawU64 := binary.BigEndian.Uint64(h[:8])
	// Mask sign bit to stay positive when cast to int64.
	seed := int64(rawU64 & 0x7FFFFFFFFFFFFFFF) //nolint:gosec

	idx := seed % int64(len(answers))
	solution := answers[idx]

	solHash := sha256.Sum256([]byte(solution))

	puzzle := store.DailyPuzzle{
		ID:            date,
		GameID:        "wordle",
		Seed:          seed,
		Solution:      solution,
		SolutionHash:  hex.EncodeToString(solHash[:]),
		SchemaVersion: 1,
	}
	if err := repo.Upsert(ctx, puzzle); err != nil {
		return "", fmt.Errorf("wordle: EnsureToday upsert: %w", err)
	}
	return solution, nil
}
