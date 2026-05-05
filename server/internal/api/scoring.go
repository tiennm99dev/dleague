package api

import (
	"strings"

	"github.com/tiennm99/dleague/server/internal/store"
)

// Wordle-style scoring tunables. Numbers picked to roughly match the
// solo-game intuition: a first-guess solve is the cap, six guesses for a
// win is still positive, a loss is zero. Wins beyond MaxGuesses don't count.
const (
	scoreFirstSolve = 100
	scorePerGuess   = 15
	scoreFloor      = 0
	MaxGuesses      = 6
)

// Score evaluates an attempt against a puzzle. Pure; no state.
//
// `guesses` are the user-submitted attempts in order. `solution` is the
// puzzle's word; case-insensitive equality wins. Only the first MaxGuesses
// are considered — a "win" on the 7th guess or later is not a win. The
// caller passes only the per-guess fields populated; this function does not
// trust any score the client provided.
func Score(solution string, guesses []string) (score int64, won bool) {
	target := strings.ToLower(strings.TrimSpace(solution))
	if target == "" || len(guesses) == 0 {
		return scoreFloor, false
	}

	for i, g := range guesses {
		if i >= MaxGuesses {
			break
		}
		if strings.ToLower(strings.TrimSpace(g)) == target {
			n := scoreFirstSolve - i*scorePerGuess
			if n < scoreFloor {
				n = scoreFloor
			}
			return int64(n), true
		}
	}
	return scoreFloor, false
}

// finalize returns the attempt with score and won re-derived server-side.
// The client never sets these fields; this function overrides whatever was
// posted.
func finalize(a store.Attempt, solution string) store.Attempt {
	score, won := Score(solution, a.Guesses)
	a.Score = score
	a.Won = won
	a.InProgress = !won && len(a.Guesses) < MaxGuesses
	return a
}
