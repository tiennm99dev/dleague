package wordle

import (
	"errors"
	"strings"

	"github.com/tiennm99/dleague/shared/game"
	dleaguev1 "github.com/tiennm99/dleague/shared/pb/dleague/v1"
)

const (
	// MaxAttempts is the maximum number of guesses a player may submit.
	MaxAttempts = 6
	// WordLen is the required word length for guesses and solutions.
	WordLen = 5
)

// Sentinel errors for validation — exported so handlers can check with errors.Is.
var (
	ErrWrongLength    = errors.New("wordle: guess must be exactly 5 letters")
	ErrNotInDictionary = errors.New("wordle: guess not in word list")
	ErrGameOver       = errors.New("wordle: game is already over")
)

// Wordle holds the mutable state of one server-authoritative Wordle session.
// solution is never sent to the client until the game is terminal.
type Wordle struct {
	solution          string   // upper-case 5-letter target word
	guesses           []string // submitted guesses in order
	hints             [][]Color
	attemptsRemaining int
	won               bool
	lost              bool
}

// New creates a fresh Wordle session for the given solution.
// solution must be exactly WordLen upper-case ASCII letters.
func New(solution string) *Wordle {
	return &Wordle{
		solution:          strings.ToUpper(solution),
		attemptsRemaining: MaxAttempts,
	}
}

// Validate checks whether the guess is acceptable without mutating state.
// Both length and dictionary checks are performed before returning any error
// to avoid leaking timing about which check failed.
func (w *Wordle) Validate(guess string, dict []string) error {
	// Normalise to upper-case before length check — consistent with Apply.
	upper := strings.ToUpper(guess)

	// Length and dictionary checks both evaluate before returning so callers
	// cannot distinguish which failed by measuring response time.
	lengthOK := len(upper) == WordLen
	inDict := contains(dict, upper)

	if !lengthOK {
		return ErrWrongLength
	}
	if !inDict {
		return ErrNotInDictionary
	}
	return nil
}

// Apply records the guess, scores it, and advances game state.
// Call Validate before Apply; Apply does not re-validate.
func (w *Wordle) Apply(guess string) error {
	if w.IsTerminal() {
		return ErrGameOver
	}
	upper := strings.ToUpper(guess)
	colors := Score(upper, w.solution)
	w.guesses = append(w.guesses, upper)
	w.hints = append(w.hints, colors)
	w.attemptsRemaining--

	if upper == w.solution {
		w.won = true
	} else if w.attemptsRemaining == 0 {
		w.lost = true
	}
	return nil
}

// IsTerminal reports whether the game has reached a terminal state.
func (w *Wordle) IsTerminal() bool {
	return w.won || w.lost
}

// Result returns a game.Result; only valid when IsTerminal() is true.
func (w *Wordle) Result() game.Result {
	return game.Result{
		Won:          w.won,
		AttemptsUsed: len(w.guesses),
	}
}

// ToProto converts the current state to a wire-safe WordleState proto.
// solution is only populated when the game is terminal.
func (w *Wordle) ToProto() *dleaguev1.WordleState {
	hints := make([]*dleaguev1.WordleHint, len(w.hints))
	for i, h := range w.hints {
		colors := make([]dleaguev1.Color, len(h))
		copy(colors, h)
		hints[i] = &dleaguev1.WordleHint{Colors: colors}
	}
	state := &dleaguev1.WordleState{
		Guesses:           append([]string(nil), w.guesses...),
		Hints:             hints,
		AttemptsRemaining: int32(w.attemptsRemaining), //nolint:gosec
		Won:               w.won,
		Lost:              w.lost,
	}
	// Reveal solution only on terminal state to prevent client-side cheating.
	if w.IsTerminal() {
		state.Solution = w.solution
	}
	return state
}

// contains is a simple O(n) linear search. Dictionaries are small enough
// (<5000 words) that a map is unnecessary for MVP; Phase 10 can optimise.
func contains(dict []string, word string) bool {
	for _, w := range dict {
		if w == word {
			return true
		}
	}
	return false
}
