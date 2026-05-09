// Client-side Wordle game — optimistic preview implementation.
// Mirrors server/internal/game/wordle/wordle.go.
// SERVER IS AUTHORITATIVE: this is used only for instant UI feedback before
// the server response arrives. The server reply always overrides local state.

import type { Game, Move, Result, State } from '../game';
import { score, WORD_LEN, type Color } from './colors';

export const MAX_ATTEMPTS = 6;

// ── Types ─────────────────────────────────────────────────────────────────────

export interface WordleMove extends Move {
	readonly guess: string;
}

export interface WordleState extends State {
	readonly guesses: readonly string[];
	readonly hints: readonly (readonly Color[])[];
	readonly attemptsRemaining: number;
	readonly won: boolean;
	readonly lost: boolean;
	// solution only populated when isTerminal (server-authoritative reveal).
	readonly solution: string;
}

// ── Initial state ─────────────────────────────────────────────────────────────

/** createInitialState returns a fresh game state for optimistic preview. */
export function createInitialState(): WordleState {
	return {
		guesses: [],
		hints: [],
		attemptsRemaining: MAX_ATTEMPTS,
		won: false,
		lost: false,
		solution: '',
		get isTerminal(): boolean {
			return this.won || this.lost;
		}
	};
}

// ── Game implementation ───────────────────────────────────────────────────────

/** WordleGame implements the pluggable Game interface for client-side preview. */
export class WordleGame implements Game<WordleState, WordleMove> {
	private readonly dictionary: ReadonlySet<string>;

	constructor(dictionary: readonly string[]) {
		this.dictionary = new Set(dictionary.map((w) => w.toUpperCase()));
	}

	validate(state: WordleState, move: WordleMove): string | null {
		if (state.isTerminal) return 'Game is already over';
		const upper = move.guess.toUpperCase();
		if (upper.length !== WORD_LEN) return `Guess must be exactly ${WORD_LEN} letters`;
		if (!this.dictionary.has(upper)) return 'Not in word list';
		return null;
	}

	apply(state: WordleState, move: WordleMove): WordleState {
		const upper = move.guess.toUpperCase();
		// solution is '' for optimistic preview (unknown client-side); use empty
		// string so hints are all gray — server reply will correct them.
		const hint = state.solution ? score(upper, state.solution) : new Array<Color>(WORD_LEN).fill('gray');
		const newGuesses = [...state.guesses, upper];
		const newHints = [...state.hints, hint];
		const remaining = state.attemptsRemaining - 1;
		const won = state.solution !== '' && upper === state.solution;
		const lost = !won && remaining === 0;

		return {
			guesses: newGuesses,
			hints: newHints,
			attemptsRemaining: remaining,
			won,
			lost,
			solution: state.solution,
			get isTerminal(): boolean {
				return this.won || this.lost;
			}
		};
	}

	result(state: WordleState): Result {
		return {
			won: state.won,
			attemptsUsed: state.guesses.length
		};
	}
}
