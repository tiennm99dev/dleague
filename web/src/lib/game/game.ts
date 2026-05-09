// game.ts: local game-state mirror used by Wordle. Extension points exist for future games
// but the v2 multi-game scaffold is not active in this release. See
// plans/260509-1331-improvement-plan/phase-04-pluggability-decision.md.
// Server is always authoritative; client implementations are for optimistic
// UI preview only and must never be treated as ground truth.

/** Base state interface every game state must satisfy. */
export interface State {
	readonly isTerminal: boolean;
}

/** Base move interface; concrete games extend with their own payload. */
// eslint-disable-next-line @typescript-eslint/no-empty-object-type
export interface Move {}

/** Terminal game result. */
export interface Result {
	readonly won: boolean;
	readonly attemptsUsed: number;
}

/**
 * Game is the client-side interface for optimistic preview (v2 scaffold; currently only Wordle is active).
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
