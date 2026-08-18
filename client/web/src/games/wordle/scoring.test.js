import { describe, it, expect } from 'vitest';
import { evaluateGuess, isWin, score } from './scoring';

describe('evaluateGuess', () => {
  it('marks all hits when guess equals solution', () => {
    expect(evaluateGuess('CRANE', 'CRANE')).toEqual([
      'hit', 'hit', 'hit', 'hit', 'hit',
    ]);
  });

  it('marks all misses when no letters overlap', () => {
    expect(evaluateGuess('XYZQQ', 'CRANE')).toEqual([
      'miss', 'miss', 'miss', 'miss', 'miss',
    ]);
  });

  it('marks present for right letter wrong slot', () => {
    expect(evaluateGuess('NACRE', 'CRANE')).toEqual([
      'present', 'present', 'present', 'present', 'hit',
    ]);
  });

  it('handles duplicate letters: greens consume target slots first', () => {
    // solution ALLOY: one A, two L's. Guess LLAMA: green at index 1 (L),
    // L at index 0 is present (matches the second L), A at index 2 is
    // present (matches the only A), M at 3 miss, A at 4 miss (no A left).
    expect(evaluateGuess('LLAMA', 'ALLOY')).toEqual([
      'present', 'hit', 'present', 'miss', 'miss',
    ]);
  });

  it('does not over-yellow when target only has one of a letter', () => {
    // solution CRANE has one E. Guess EERIE: third E is the hit at index
    // 4? No, CRANE is C R A N E, so E is at slot 4. EERIE has E at 0,1,3,4.
    // Greens first: E at 4 = hit. Remaining target counts: C=1, R=1, A=1, N=1.
    // Then go left to right: E at 0 → no E remaining → miss. E at 1 → miss.
    // R at 2 → present (R count 1 → 0). I at 3 → miss. So:
    expect(evaluateGuess('EERIE', 'CRANE')).toEqual([
      'miss', 'miss', 'present', 'miss', 'hit',
    ]);
  });

  it('throws on length mismatch', () => {
    expect(() => evaluateGuess('AB', 'ABCDE')).toThrow();
  });

  it('is case-insensitive', () => {
    expect(evaluateGuess('crane', 'CRANE')).toEqual([
      'hit', 'hit', 'hit', 'hit', 'hit',
    ]);
  });
});

describe('isWin', () => {
  it('true for all hits', () => {
    expect(isWin(['hit', 'hit', 'hit'])).toBe(true);
  });

  it('false on any non-hit', () => {
    expect(isWin(['hit', 'present', 'hit'])).toBe(false);
    expect(isWin(['hit', 'miss', 'hit'])).toBe(false);
  });

  it('false on empty', () => {
    expect(isWin([])).toBe(false);
  });
});

describe('score', () => {
  it('100 for first-guess solve', () => {
    expect(score(['CRANE'], 'CRANE')).toEqual({ score: 100, won: true });
  });

  it('drops 15 per guess until solve', () => {
    expect(score(['WRONG', 'CRANE'], 'CRANE')).toEqual({ score: 85, won: true });
    expect(score(['A', 'B', 'C', 'D', 'E', 'CRANE'], 'CRANE')).toEqual({
      score: 25, won: true,
    });
  });

  it('zero when never solves', () => {
    expect(score(['WRONG', 'AGAIN'], 'CRANE')).toEqual({ score: 0, won: false });
  });

  it('does not award for solve beyond MAX_GUESSES', () => {
    const guesses = ['A', 'B', 'C', 'D', 'E', 'F', 'CRANE'];
    expect(score(guesses, 'CRANE')).toEqual({ score: 0, won: false });
  });
});
