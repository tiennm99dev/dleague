// @vitest-environment happy-dom
// Tests for connection-status.svelte: Reconnect button visibility/disabled state.
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/svelte';

// vi.hoisted runs before mock hoisting, so these values are available in vi.mock factories.
// Use a plain object with a subscribe method to avoid ESM import inside hoisted callback.
const { mockConnectionState, mockConnect, mockIdToken } = vi.hoisted(() => {
	type State = 'disconnected' | 'connecting' | 'connected';
	let _state: State = 'connected';
	const _subs = new Set<(v: State) => void>();
	const store = {
		subscribe(fn: (v: State) => void) {
			fn(_state);
			_subs.add(fn);
			return () => _subs.delete(fn);
		},
		set(v: State) {
			_state = v;
			_subs.forEach((fn) => fn(v));
		}
	};
	return {
		mockConnectionState: store,
		mockConnect: vi.fn(),
		mockIdToken: vi.fn().mockResolvedValue('tok')
	};
});

vi.mock('$lib/ws', () => ({
	connectionState: mockConnectionState,
	connect: mockConnect
}));
vi.mock('$lib/auth-store', () => ({
	idToken: mockIdToken
}));

import ConnectionStatus from './connection-status.svelte';

describe('ConnectionStatus', () => {
	it('shows Reconnect button when state is disconnected', () => {
		mockConnectionState.set('disconnected');
		render(ConnectionStatus, { props: {} });
		expect(screen.getByRole('button', { name: 'Reconnect' })).toBeTruthy();
	});

	it('Reconnect button is not disabled when disconnected', () => {
		mockConnectionState.set('disconnected');
		render(ConnectionStatus, { props: {} });
		const btn = screen.getByRole('button', { name: 'Reconnect' });
		expect((btn as HTMLButtonElement).disabled).toBe(false);
	});

	it('does not show Reconnect button when state is connected', () => {
		mockConnectionState.set('connected');
		render(ConnectionStatus, { props: {} });
		expect(screen.queryByRole('button', { name: 'Reconnect' })).toBeNull();
	});

	it('does not show Reconnect button when state is connecting', () => {
		mockConnectionState.set('connecting');
		render(ConnectionStatus, { props: {} });
		expect(screen.queryByRole('button', { name: 'Reconnect' })).toBeNull();
	});
});
