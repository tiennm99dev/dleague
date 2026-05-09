// @vitest-environment happy-dom
// Tests for keyboard.svelte: tabindex, key emission, canonical casing.
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';
import Keyboard from './keyboard.svelte';

describe('Keyboard', () => {
	it('all key buttons have tabindex="-1"', () => {
		const { container } = render(Keyboard, {
			props: { hints: [], guesses: [], onkey: vi.fn() }
		});

		const buttons = container.querySelectorAll('button');
		expect(buttons.length).toBeGreaterThan(0);
		for (const btn of buttons) {
			expect(btn.getAttribute('tabindex')).toBe('-1');
		}
	});

	it('clicking a letter key calls onkey with uppercase letter', async () => {
		const onkey = vi.fn();
		render(Keyboard, {
			props: { hints: [], guesses: [], onkey }
		});

		const aBtn = screen.getByRole('button', { name: 'A' });
		await fireEvent.click(aBtn);

		expect(onkey).toHaveBeenCalledWith('A');
	});

	it('clicking Enter key emits canonical "Enter"', async () => {
		const onkey = vi.fn();
		render(Keyboard, {
			props: { hints: [], guesses: [], onkey }
		});

		const enterBtn = screen.getByRole('button', { name: 'Enter' });
		await fireEvent.click(enterBtn);

		expect(onkey).toHaveBeenCalledWith('Enter');
	});

	it('clicking Backspace key emits canonical "Backspace"', async () => {
		const onkey = vi.fn();
		render(Keyboard, {
			props: { hints: [], guesses: [], onkey }
		});

		const backspaceBtn = screen.getByRole('button', { name: 'Backspace' });
		await fireEvent.click(backspaceBtn);

		expect(onkey).toHaveBeenCalledWith('Backspace');
	});

	it('applies green class to a correctly guessed letter', () => {
		const { container } = render(Keyboard, {
			props: {
				hints: [['green', 'gray', 'gray', 'gray', 'gray']],
				guesses: ['CRANE'],
				onkey: vi.fn()
			}
		});

		// 'C' is at position 0 with green hint — button should have key--green class.
		const cBtn = container.querySelector('button[aria-label="C"]');
		expect(cBtn?.classList.contains('key--green')).toBe(true);
	});
});
