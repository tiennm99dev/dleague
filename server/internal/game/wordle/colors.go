// Package wordle implements the server-authoritative Wordle game logic.
// The two-pass colour algorithm here is the canonical implementation that
// correctly handles repeated letters in both guess and solution.
package wordle

import dleaguev1 "github.com/tiennm99/dleague/shared/pb/dleague/v1"

// Color is a package-level alias so callers don't import the pb package
// directly for game logic.
type Color = dleaguev1.Color

const (
	ColorGray   = dleaguev1.Color_COLOR_GRAY
	ColorYellow = dleaguev1.Color_COLOR_YELLOW
	ColorGreen  = dleaguev1.Color_COLOR_GREEN
)

// Score computes the per-letter color result for guess against solution.
//
// Algorithm — two passes:
//  1. Pass 1: mark GREENs. Consume the matching solution slot so it cannot
//     yield a YELLOW for the same position.
//  2. Pass 2: for each non-GREEN guess letter, scan unconsumed solution slots
//     left-to-right; first match → YELLOW, consume slot.
//
// This correctly handles:
//   - Repeated letter in guess but only one in solution → at most one
//     YELLOW/GREEN for that letter.
//   - Repeated letter in both → as many GREENs/YELLOWs as there are matches.
//
// Both guess and solution must be exactly WordLen characters (upper-case).
// Callers are responsible for normalising case before calling Score.
func Score(guess, solution string) []Color {
	result := make([]Color, WordLen)
	// consumed tracks which solution positions have been claimed by a GREEN or YELLOW.
	consumed := make([]bool, WordLen)

	// Pass 1 — greens.
	for i := 0; i < WordLen; i++ {
		if guess[i] == solution[i] {
			result[i] = ColorGreen
			consumed[i] = true
		}
	}

	// Pass 2 — yellows for remaining positions.
	for i := 0; i < WordLen; i++ {
		if result[i] == ColorGreen {
			continue
		}
		// Search for an unconsumed solution slot containing this letter.
		for j := 0; j < WordLen; j++ {
			if !consumed[j] && guess[i] == solution[j] {
				result[i] = ColorYellow
				consumed[j] = true
				break
			}
		}
		// If no match was found, result[i] remains COLOR_UNSPECIFIED (0).
		// Normalise to GRAY explicitly for clarity in downstream consumers.
		if result[i] == dleaguev1.Color_COLOR_UNSPECIFIED {
			result[i] = ColorGray
		}
	}

	return result
}
