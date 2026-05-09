// Client-side Wordle colour algorithm — mirrors server/internal/game/wordle/colors.go.
// Used for optimistic UI preview only; server response is always authoritative.
// Two-pass algorithm handles repeated letters correctly.

export const WORD_LEN = 5;

export type Color = 'green' | 'yellow' | 'gray';

/**
 * score computes the per-letter colour result for guess against solution.
 *
 * Pass 1 — mark GREENs, consume matching solution slots.
 * Pass 2 — for each non-green guess letter, scan unconsumed solution slots;
 *           first match → YELLOW, consume slot; no match → GRAY.
 *
 * Both guess and solution must be exactly WORD_LEN characters (upper-case).
 * Callers normalise case before calling score.
 */
export function score(guess: string, solution: string): Color[] {
	const result: Color[] = new Array(WORD_LEN).fill('gray');
	const consumed: boolean[] = new Array(WORD_LEN).fill(false);

	// Pass 1 — greens.
	for (let i = 0; i < WORD_LEN; i++) {
		if (guess[i] === solution[i]) {
			result[i] = 'green';
			consumed[i] = true;
		}
	}

	// Pass 2 — yellows.
	for (let i = 0; i < WORD_LEN; i++) {
		if (result[i] === 'green') continue;
		for (let j = 0; j < WORD_LEN; j++) {
			if (!consumed[j] && guess[i] === solution[j]) {
				result[i] = 'yellow';
				consumed[j] = true;
				break;
			}
		}
		// result[i] stays 'gray' if no match found (already initialised above).
	}

	return result;
}
