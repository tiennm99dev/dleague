// Mirrors server/internal/game/wordle/colors_test.go — same canonical cases.
// These tests verify the client-side optimistic scoring matches the server.
import { describe, it, expect } from 'vitest';
import { score } from './colors';
import type { Color } from './colors';

function c(s: string): Color[] {
	return s.split('').map((ch): Color => {
		if (ch === 'G') return 'green';
		if (ch === 'Y') return 'yellow';
		return 'gray';
	});
}

describe('score', () => {
	it('all green — perfect match', () => {
		expect(score('CRANE', 'CRANE')).toEqual(c('GGGGG'));
	});

	it('all gray — no letters match', () => {
		expect(score('XXXXX', 'CRANE')).toEqual(c('_____'));
	});

	it('SHARP/SHRED — two greens, one yellow', () => {
		// SHRED vs SHARP: S=G H=G R=Y E=_ D=_
		expect(score('SHRED', 'SHARP')).toEqual(c('GGY__'));
	});

	it('EERIE/ALLEE — green at pos4, yellow at pos3 (one E consumed)', () => {
		// ALLEE vs EERIE:
		// pass1: pos4 E=E → green; consume sol[4]
		// pass2: pos3 E → sol[0]=E unconsumed → yellow
		expect(score('ALLEE', 'EERIE')).toEqual(c('___YG'));
	});

	it('AROMA/AAAAA — two greens at A positions, rest gray', () => {
		// AAAAA vs AROMA:
		// pass1: pos0 A=A→green, pos4 A=A→green
		// pass2: pos1,2,3 A — only R,O,M left → gray
		expect(score('AAAAA', 'AROMA')).toEqual(c('G___G'));
	});

	it('LLAMA/SPELL — two L positions in solution yield two yellows', () => {
		// SPELL has L at pos3 and pos4; LLAMA has L at pos0 and pos1
		// pass1: no greens
		// pass2: pos0 L → sol[3]=L → yellow; pos1 L → sol[4]=L → yellow
		expect(score('LLAMA', 'SPELL')).toEqual(c('YY___'));
	});

	it('CRANE/TRACE — yellow C, two greens, gray N, green E', () => {
		// pass1: pos1 R=R, pos2 A=A, pos4 E=E → greens; consume sol[1,2,4]
		// pass2: pos0 C → sol[3]=C → yellow; pos3 N → no match → gray
		expect(score('CRANE', 'TRACE')).toEqual(c('YGG_G'));
	});

	it('RATES/TASER — mixed pattern with all 5 letters present', () => {
		// RATES vs TASER: R A T E S all in TASER, positions shift
		// pass1: pos1 A=A→green, pos3 E=E→green; consume sol[1,3]
		// pass2: pos0 R→sol[4]=R→yellow; pos2 T→sol[0]=T→yellow; pos4 S→sol[2]=S→yellow
		expect(score('RATES', 'TASER')).toEqual(c('YGYGY'));
	});

	it('ENTER/ETHER — two matching Es plus one misplaced T', () => {
		// ENTER vs ETHER:
		// pass1: pos0 E=E→green, pos3 E=E→green, pos4 R=R→green; consume sol[0,3,4]
		// pass2: pos1 N→no match→gray; pos2 T→sol[1]=T→yellow
		expect(score('ENTER', 'ETHER')).toEqual(c('G_YGG'));
	});
});
