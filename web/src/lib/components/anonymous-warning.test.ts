// @vitest-environment happy-dom
// Tests for anonymous-warning.svelte: inline vs banner variants.
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import AnonWarning from './anonymous-warning.svelte';

describe('AnonWarning', () => {
	it('inline=true renders a <small> element', () => {
		const { container } = render(AnonWarning, {
			props: { inline: true }
		});
		expect(container.querySelector('small')).toBeTruthy();
	});

	it('inline=false renders a banner with role="status"', () => {
		render(AnonWarning, { props: { inline: false } });
		expect(screen.getByRole('status')).toBeTruthy();
	});

	it('defaults to banner (inline=false)', () => {
		render(AnonWarning, { props: {} });
		expect(screen.getByRole('status')).toBeTruthy();
	});

	it('inline banner contains sign-in mention', () => {
		render(AnonWarning, { props: { inline: true } });
		expect(screen.getByText(/sign in/i)).toBeTruthy();
	});
});
