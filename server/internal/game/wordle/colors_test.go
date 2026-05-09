package wordle

import (
	"testing"
)

// colorSlice converts a human-readable string "GGYrG" (G=green, Y=yellow, r=gray)
// into a []Color slice for compact test-table notation.
// G = green, Y = yellow, any other char = gray.
func colorSlice(s string) []Color {
	out := make([]Color, len(s))
	for i, c := range s {
		switch c {
		case 'G':
			out[i] = ColorGreen
		case 'Y':
			out[i] = ColorYellow
		default:
			out[i] = ColorGray
		}
	}
	return out
}

func colorsEqual(a, b []Color) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestScore(t *testing.T) {
	tests := []struct {
		name     string
		guess    string
		solution string
		want     string // G=green Y=yellow _=gray (5 chars)
	}{
		{
			// All greens — perfect match.
			name:     "all_green",
			guess:    "CRANE",
			solution: "CRANE",
			want:     "GGGGG",
		},
		{
			// All misses.
			name:     "all_gray",
			guess:    "XXXXX",
			solution: "CRANE",
			want:     "_____",
		},
		{
			// Canonical repeated-letter case: SHARP/SHRED.
			// S=green H=green R=gray E=gray D=green (positions 0,1,4 match).
			// Wait — let's verify manually:
			// SHARP: S H A R P
			// SHRED: S H R E D
			// pos0: S==S → green
			// pos1: H==H → green
			// pos2: R vs A → R is in SHARP at pos3 → yellow
			// pos3: E vs R → E not in SHARP → gray
			// pos4: D vs P → D not in SHARP → gray
			name:     "SHARP_SHRED",
			guess:    "SHRED",
			solution: "SHARP",
			want:     "GGY__",
		},
		{
			// EERIE / ALLEE — only one yellow E (solution has 3 E's but
			// ALLEE has two E's: pos3 is green because solution[3]=R? No.
			// Let's hand-verify:
			// EERIE: E E R I E  (solution)
			// ALLEE: A L L E E  (guess)
			// Pass1 greens:
			//   pos0: A vs E → no
			//   pos1: L vs E → no
			//   pos2: L vs R → no
			//   pos3: E vs I → no
			//   pos4: E vs E → GREEN; consume solution[4]
			// Pass2 yellows (consumed = [false,false,false,false,true]):
			//   pos0: A — not in any unconsumed solution slot → gray
			//   pos1: L — not in any unconsumed solution slot → gray
			//   pos2: L — not in any unconsumed solution slot → gray
			//   pos3: E — solution[0]=E unconsumed → YELLOW; consume [0]
			//   pos4: already green
			// Result: _ _ _ Y G
			name:     "EERIE_ALLEE",
			guess:    "ALLEE",
			solution: "EERIE",
			want:     "___YG",
		},
		{
			// AROMA / AAAAA — repeated letter in guess, solution has 2 A's (pos0, pos4).
			// Pass1 greens:
			//   pos0: A==A → green; consume sol[0]
			//   pos1: A vs R → no
			//   pos2: A vs O → no
			//   pos3: A vs M → no
			//   pos4: A==A → green; consume sol[4]
			// Pass2 (consumed=[true,false,false,false,true]):
			//   pos1: A — remaining unconsumed sol slots: R,O,M — no A → gray
			//   pos2: A — same → gray
			//   pos3: A — same → gray
			// Result: G _ _ _ G
			name:     "AROMA_AAAAA",
			guess:    "AAAAA",
			solution: "AROMA",
			want:     "G___G",
		},
		{
			// Repeated letter in guess, TWO in solution — both get yellows.
			// LLAMA / SPELL: SPELL has L at pos3 AND pos4.
			// Pass1: no greens (L≠S, L≠P, A≠E, M≠L, A≠L)
			// Pass2:
			//   pos0: L → sol[3]=L unconsumed → YELLOW; consume [3]
			//   pos1: L → sol[4]=L unconsumed → YELLOW; consume [4]
			//   pos2: A → not in SPELL → gray
			//   pos3: M → not in SPELL → gray
			//   pos4: A → not in SPELL → gray
			// Result: Y Y _ _ _
			name:     "two_yellows_for_two_solution_ls",
			guess:    "LLAMA",
			solution: "SPELL",
			want:     "YY___",
		},
		{
			// C in TRACE is at pos3; N is not in TRACE.
			// CRANE vs TRACE:
			// Pass1 greens: pos1 R=R, pos2 A=A, pos4 E=E; consume sol[1,2,4]
			// Pass2: pos0 C → sol[3]=C unconsumed → YELLOW; pos3 N → no match → gray
			// Result: Y G G _ G
			name:     "single_yellow_crane_trace",
			guess:    "CRANE",
			solution: "TRACE",
			want:     "YGG_G",
		},
		{
			// All yellows — every letter present but all wrong positions.
			// RATES / TASER: R,A,T,E,S all in TASER, none in correct position.
			// RATES: R A T E S
			// TASER: T A S E R
			// Pass1: pos1: A==A → green; pos3: E==E → green
			// Pass2 (consumed [false,true,false,false,false] wait sol is TASER):
			// sol = T A S E R  (indices 0..4)
			// Pass1:
			//   pos0: R vs T → no
			//   pos1: A vs A → green; consume sol[1]
			//   pos2: T vs S → no
			//   pos3: E vs E → green; consume sol[3]
			//   pos4: S vs R → no
			// Pass2 (consumed=[false,true,false,true,false]):
			//   pos0: R — check sol: T(no), skip[1], S(no), skip[3], R(yes@4)→YELLOW; consume[4]
			//   pos2: T — check sol: T(yes@0)→YELLOW; consume[0]
			//   pos4: S — check sol: skip[0]consumed, skip[1], S(yes@2)→YELLOW; consume[2]
			// Result: Y G Y G Y
			name:     "RATES_TASER",
			guess:    "RATES",
			solution: "TASER",
			want:     "YGYGY",
		},
		{
			// Letter appears twice in solution, once in guess at correct pos.
			// Guess ENTER, solution ETHER: E at pos0 is green; second E not in guess.
			// ENTER: E N T E R
			// ETHER: E T H E R
			// Pass1: pos0:E==E→green, pos3:E==E→green, pos4:R==R→green; consume sol[0,3,4]
			// Pass2 (consumed=[true,false,false,true,true]):
			//   pos1: N — sol[1]=T, sol[2]=H → no N → gray
			//   pos2: T — sol[1]=T unconsumed → YELLOW; consume[1]
			// Result: G _ Y G G
			name:     "ENTER_ETHER",
			guess:    "ENTER",
			solution: "ETHER",
			want:     "G_YGG",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Score(tc.guess, tc.solution)
			want := colorSlice(tc.want)
			if !colorsEqual(got, want) {
				t.Errorf("Score(%q, %q) = %v, want %v", tc.guess, tc.solution, got, want)
			}
		})
	}
}
