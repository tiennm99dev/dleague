// Pure Wordle scoring. Server is the trust boundary — this code only drives
// per-guess UX feedback. Final score reverification lives in
// server/internal/api/scoring.go.

/** @typedef {'hit' | 'present' | 'miss'} LetterEval */

export const MAX_GUESSES = 6;

// evaluateGuess returns one Eval per letter following Wordle's
// duplicate-letter rule: greens are placed first, then yellows are issued
// only against remaining (non-green) target letters. So `LLAMA` against
// solution `ALLOY` yields [present, hit, present, miss, miss], not
// [present, hit, present, present, miss].
/**
 * @param {string} guess
 * @param {string} solution
 * @returns {LetterEval[]}
 */
export function evaluateGuess(guess, solution) {
  const g = guess.toUpperCase();
  const s = solution.toUpperCase();
  if (g.length !== s.length) {
    throw new Error(`guess length ${g.length} != solution length ${s.length}`);
  }
  /** @type {LetterEval[]} */
  const out = new Array(g.length).fill('miss');
  /** @type {Record<string, number>} */
  const remaining = {};

  for (let i = 0; i < g.length; i++) {
    if (g[i] === s[i]) {
      out[i] = 'hit';
    } else {
      remaining[s[i]] = (remaining[s[i]] ?? 0) + 1;
    }
  }
  for (let i = 0; i < g.length; i++) {
    if (out[i] === 'hit') continue;
    const c = g[i];
    if ((remaining[c] ?? 0) > 0) {
      out[i] = 'present';
      remaining[c] -= 1;
    }
  }
  return out;
}

/**
 * @param {LetterEval[]} evaluation
 * @returns {boolean}
 */
export function isWin(evaluation) {
  return evaluation.length > 0 && evaluation.every((e) => e === 'hit');
}

// Mirror the server's score formula (server/internal/api/scoring.go) so the
// HUD can show a preview that matches the canonical leaderboard score.
/**
 * @param {string[]} guesses
 * @param {string} solution
 * @returns {{ score: number, won: boolean }}
 */
export function score(guesses, solution) {
  const SCORE_FIRST_SOLVE = 100;
  const SCORE_PER_GUESS = 15;
  const target = solution.trim().toUpperCase();
  if (!target || guesses.length === 0) return { score: 0, won: false };
  for (let i = 0; i < guesses.length && i < MAX_GUESSES; i++) {
    if (guesses[i].trim().toUpperCase() === target) {
      const n = Math.max(0, SCORE_FIRST_SOLVE - i * SCORE_PER_GUESS);
      return { score: n, won: true };
    }
  }
  return { score: 0, won: false };
}
