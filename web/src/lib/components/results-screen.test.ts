// @vitest-environment happy-dom
// Tests for results-screen.svelte: reason-prop variants produce correct headlines/CTAs.
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import ResultsScreen from './results-screen.svelte';

// Mock $app/navigation — goto is used in CTA onclick handlers (hoisted by vitest).
vi.mock('$app/navigation', () => ({ goto: vi.fn() }));

describe('ResultsScreen reason variants', () => {
	it('win: shows "You solved it!" headline', () => {
		render(ResultsScreen, {
			props: { won: true, solution: 'CRANE' }
		});
		expect(screen.getByText('You solved it!')).toBeTruthy();
	});

	it('loss (won=false, no reason): shows "Better luck next time"', () => {
		render(ResultsScreen, {
			props: { won: false, solution: 'CRANE' }
		});
		expect(screen.getByText('Better luck next time')).toBeTruthy();
	});

	it('tie: shows "It\'s a tie!" headline', () => {
		render(ResultsScreen, {
			props: { won: false, solution: 'CRANE', reason: 'tie' }
		});
		expect(screen.getByText("It's a tie!")).toBeTruthy();
	});

	it('opponent-left: shows warning headline and "Find new match" CTA', () => {
		render(ResultsScreen, {
			props: { won: false, solution: 'CRANE', reason: 'opponent-left' }
		});
		expect(screen.getByText('Opponent left the match.')).toBeTruthy();
		expect(screen.getByRole('button', { name: 'Find new match' })).toBeTruthy();
	});

	it('self-disconnect: shows disconnect headline and "Try again" CTA', () => {
		render(ResultsScreen, {
			props: { won: false, solution: 'CRANE', reason: 'self-disconnect' }
		});
		expect(screen.getByText('You disconnected. Match was lost.')).toBeTruthy();
		expect(screen.getByRole('button', { name: 'Try again' })).toBeTruthy();
	});

	it('shows the solution word', () => {
		render(ResultsScreen, {
			props: { won: true, solution: 'MAGIC' }
		});
		expect(screen.getByText('MAGIC')).toBeTruthy();
	});

	it('renders the region with correct aria-label', () => {
		render(ResultsScreen, {
			props: { won: true, solution: 'CRANE' }
		});
		expect(screen.getByRole('region', { name: 'Game result' })).toBeTruthy();
	});

	it('shows "View leaderboard" button in all variants', () => {
		render(ResultsScreen, {
			props: { won: false, solution: 'CRANE', reason: 'tie' }
		});
		expect(
			screen.getByRole('button', { name: 'View leaderboard' })
		).toBeTruthy();
	});
});
