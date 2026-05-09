// @vitest-environment happy-dom
// Tests for board.svelte: ARIA structure and letter rendering.
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import Board from './board.svelte';

describe('Board', () => {
	it('renders the ARIA region with correct label', () => {
		render(Board, {
			props: { guesses: [], hints: [], currentInput: '' }
		});

		expect(screen.getByRole('region', { name: 'Wordle board' })).toBeTruthy();
	});

	it('renders 6 rows of 5 tiles', () => {
		const { container } = render(Board, {
			props: { guesses: [], hints: [], currentInput: '' }
		});

		const rows = container.querySelectorAll('.row');
		expect(rows.length).toBe(6);

		const tiles = container.querySelectorAll('.tile');
		expect(tiles.length).toBe(30);
	});

	it('shows letters from a submitted guess', () => {
		render(Board, {
			props: {
				guesses: ['CRANE'],
				hints: [['green', 'gray', 'gray', 'yellow', 'green']],
				currentInput: ''
			}
		});

		// Each letter in the guess gets an aria-label.
		expect(screen.getByLabelText('C')).toBeTruthy();
		expect(screen.getByLabelText('R')).toBeTruthy();
		expect(screen.getByLabelText('A')).toBeTruthy();
		expect(screen.getByLabelText('N')).toBeTruthy();
		expect(screen.getByLabelText('E')).toBeTruthy();
	});

	it('applies green/yellow/gray tile classes from hints', () => {
		const { container } = render(Board, {
			props: {
				guesses: ['CRANE'],
				hints: [['green', 'yellow', 'gray', 'gray', 'green']],
				currentInput: ''
			}
		});

		const tiles = container
			.querySelectorAll('.row')[0]
			.querySelectorAll('.tile');
		expect(tiles[0].classList.contains('tile--green')).toBe(true);
		expect(tiles[1].classList.contains('tile--yellow')).toBe(true);
		expect(tiles[2].classList.contains('tile--gray')).toBe(true);
		expect(tiles[4].classList.contains('tile--green')).toBe(true);
	});

	it('shows currentInput letters in the active row', () => {
		render(Board, {
			props: { guesses: [], hints: [], currentInput: 'AB' }
		});

		// The two typed letters should appear in the first row.
		expect(screen.getByLabelText('A')).toBeTruthy();
		expect(screen.getByLabelText('B')).toBeTruthy();
	});
});
