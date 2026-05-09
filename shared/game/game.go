// Package game defines the pluggable -dle game contract.
//
// Each concrete -dle (Wordle-style, music, geography, image, ...) implements
// the Game interface and registers itself with the Registry under a stable
// game ID. The client and server consume Game implementations through this
// interface only — never reach into a concrete game type.
package game

// Result captures the terminal outcome of one game session.
type Result struct {
	// WinnerUID is the Firebase UID of the winner; empty for solo games.
	WinnerUID string
	// Won is true if the player solved the puzzle within the allowed attempts.
	Won bool
	// AttemptsUsed is the number of guesses used (1-based; 0 if lost without guessing).
	AttemptsUsed int
	// DurationMS is the elapsed wall time from first guess to terminal in milliseconds.
	DurationMS int64
}

// State is the interface every concrete game state must implement.
// Concrete games define their own state struct and embed game-specific fields.
// Transports marshal/unmarshal via proto messages — State is never sent raw.
type State interface {
	// IsTerminal reports whether the game has ended (won or out of attempts).
	IsTerminal() bool
}

// Move represents a single player action. Concrete games define their own
// Move struct carrying the game-specific input (e.g. a guess word).
type Move interface{}

// Key represents a single user input event. Kept for compatibility with
// interactive key-driven games; WS-based games use typed Move instead.
type Key string

const (
	KeyEnter     Key = "Enter"
	KeyBackspace Key = "Backspace"
)

// Game is the contract every -dle implementation fulfills.
//
// Lifecycle: Init → [Validate + Apply]* → IsTerminal == true → Result.
// Concrete games need NOT implement the old HandleKey/Tick interface; those
// are removed to align with the server-authoritative WS architecture.
type Game interface {
	// Init resets the game using the given seed. Identical seeds must produce
	// identical games (deterministic puzzles required for fair PvP).
	Init(seed int64) error

	// Validate checks whether a move is legal in the current state.
	// Returns a non-nil error if the move is invalid (wrong length, not in
	// dictionary, game already terminal, etc.).
	Validate(move Move) error

	// Apply executes a validated move and returns the updated State.
	// Callers should call Validate first; Apply behaviour on invalid moves is
	// undefined.
	Apply(move Move) (State, error)

	// IsTerminal reports whether the game has ended (won or out of attempts).
	IsTerminal() bool

	// Result returns the final outcome. Calling before IsTerminal is undefined.
	Result() Result
}
