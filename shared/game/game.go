// Package game defines the pluggable -dle game contract.
//
// Each concrete -dle (Wordle-style, music, geography, image, ...) implements
// the Game interface and registers itself with the Registry under a stable
// game ID. The client and server consume Game implementations through this
// interface only — never reach into a concrete game type.
package game

// Result captures the terminal outcome of one game session.
type Result struct {
	// Won is true if the player solved the puzzle within the allowed attempts.
	Won bool
	// Attempts is the number of guesses used (1-based; 0 if Won is false and game timed out).
	Attempts int
	// DurationMS is the elapsed wall time from Init to terminal in milliseconds.
	DurationMS int64
}

// State is an opaque, JSON-serializable snapshot of one game's progress.
// Concrete games define their own state shape; transports treat it as []byte.
type State = []byte

// Key represents a single user input event. Use a small alphabet so different
// -dle types can share the input pipeline (e.g. characters, arrow keys, enter).
type Key string

const (
	KeyEnter     Key = "Enter"
	KeyBackspace Key = "Backspace"
)

// Game is the contract every -dle implementation fulfills.
//
// Lifecycle: Init -> [HandleKey | Tick]* -> IsTerminal == true -> Result.
type Game interface {
	// Init resets the game using the given seed. Identical seeds must produce
	// identical games (deterministic puzzles are required for fair PvP).
	Init(seed int64) error

	// HandleKey processes one input event. Returns true if the game state changed.
	HandleKey(k Key) bool

	// Tick advances time-based state by dtMS milliseconds. Most -dle games are
	// turn-based and may ignore this; it exists for animations / timers.
	Tick(dtMS int64)

	// State returns a serialized snapshot suitable for transport or storage.
	State() State

	// IsTerminal reports whether the game has ended (won or out of attempts).
	IsTerminal() bool

	// Result returns the final outcome. Calling before IsTerminal is undefined.
	Result() Result
}
