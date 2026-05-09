// Pluggable game interface — TypeScript mirror of shared/game/game.go.
// Server is always authoritative; client implementations are for optimistic
// UI preview only and must never be treated as ground truth.

/** Base state interface every game state must satisfy. */
export interface State {
	readonly isTerminal: boolean;
}

/** Base move interface; concrete games extend with their own payload. */
export interface Move {}

/** Terminal game result. */
export interface Result {
	readonly won: boolean;
	readonly attemptsUsed: number;
}

/**
 * Game is the client-side pluggable interface for optimistic preview.
 * Type parameters:
 *   S — concrete State subtype
 *   M — concrete Move subtype
 */
export interface Game<S extends State, M extends Move> {
	/**
	 * validate checks a move against the current state.
	 * Returns null when the move is valid; a human-readable error string otherwise.
	 */
	validate(state: S, move: M): string | null;

	/**
	 * apply executes a valid move and returns the next state.
	 * Callers must call validate first; behaviour on invalid moves is undefined.
	 */
	apply(state: S, move: M): S;

	/**
	 * result returns the terminal outcome.
	 * Calling before state.isTerminal is undefined.
	 */
	result(state: S): Result;
}
