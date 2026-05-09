// @vitest-environment happy-dom
// Tests for ws.ts: reconnect, pending rejection, token refresh, handler dedup.
// Uses vi.resetModules() + dynamic import to get fresh singleton state per test.
// MockWebSocket captures constructor args and exposes triggerClose/triggerMessage.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// ── Top-level mocks (hoisted before any import) ───────────────────────────────

// idToken mock function — returned value is overridden per-test via mockReturnValue.
const mockIdToken = vi.fn().mockResolvedValue('tok');
const mockAuthErrorSet = vi.fn();

vi.mock('$lib/auth-store', () => ({ idToken: mockIdToken }));
vi.mock('$lib/auth-error-store', () => ({
	authError: { set: mockAuthErrorSet }
}));

// ── MockWebSocket ─────────────────────────────────────────────────────────────

type WSListener = (evt: Event) => void;

class MockWebSocket {
	static CONNECTING = 0;
	static OPEN = 1;
	static CLOSING = 2;
	static CLOSED = 3;

	static instances: MockWebSocket[] = [];

	url: string;
	protocols: string[];
	readyState: number;
	binaryType: string = 'blob';

	onopen: WSListener | null = null;
	onmessage: WSListener | null = null;
	onerror: WSListener | null = null;
	onclose: WSListener | null = null;

	sentData: unknown[] = [];

	constructor(url: string, protocols?: string | string[]) {
		this.url = url;
		this.protocols = Array.isArray(protocols)
			? protocols
			: protocols
				? [protocols]
				: [];
		this.readyState = MockWebSocket.CONNECTING;
		MockWebSocket.instances.push(this);
	}

	send(data: unknown): void {
		this.sentData.push(data);
	}

	close(_code?: number, _reason?: string): void {
		this.readyState = MockWebSocket.CLOSING;
	}

	/** Simulate the server accepting the WS handshake. */
	triggerOpen(): void {
		this.readyState = MockWebSocket.OPEN;
		this.onopen?.(new Event('open'));
	}

	/** Simulate receiving a binary message. */
	triggerMessage(data: ArrayBuffer): void {
		const evt = Object.assign(new Event('message'), { data });
		this.onmessage?.(evt as unknown as Event);
	}

	/** Simulate an abnormal close. */
	triggerClose(code = 1000, reason = ''): void {
		this.readyState = MockWebSocket.CLOSED;
		const evt = Object.assign(new Event('close'), {
			code,
			reason,
			wasClean: code === 1000
		});
		this.onclose?.(evt as unknown as Event);
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function installMockWS(): void {
	MockWebSocket.instances = [];
	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	(globalThis as any).WebSocket = MockWebSocket;
	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	(globalThis as any).WebSocket.CONNECTING = 0;
	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	(globalThis as any).WebSocket.OPEN = 1;
	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	(globalThis as any).WebSocket.CLOSING = 2;
	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	(globalThis as any).WebSocket.CLOSED = 3;
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('ws connect', () => {
	beforeEach(() => {
		vi.resetModules();
		installMockWS();
		vi.useFakeTimers();
		mockIdToken.mockResolvedValue('tok');
		mockAuthErrorSet.mockClear();
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	it('(a) connect opens WebSocket with token in Sec-WebSocket-Protocol', async () => {
		const { connect } = await import('./ws');
		connect('tok123');

		const ws = MockWebSocket.instances[0];
		expect(ws).toBeDefined();
		expect(ws.protocols).toContain('dleague.v1');
		expect(ws.protocols).toContain('fb.tok123');
	});

	it('(b) onclose rejects all pending sendRequest promises with connection-lost error', async () => {
		const { connect, sendRequest, disconnect } = await import('./ws');
		connect('tok');

		const ws = MockWebSocket.instances[0];
		ws.triggerOpen();

		// Kick off a sendRequest — will pend until resolved or rejected.
		// Use a very long timeout so it doesn't self-reject before we trigger close.
		// MessageType value 1 = any valid type to create a pending map entry.
		let rejectedErr: Error | null = null;
		const pendingPromise = sendRequest(1, new Uint8Array(0), 60_000).catch(
			(e: Error) => (rejectedErr = e)
		);

		// Trigger close — should reject all pending immediately.
		ws.triggerClose(1000, 'server going away');

		await pendingPromise;

		expect(rejectedErr).toBeTruthy();
		expect(rejectedErr!.message).toMatch(/connection lost/i);

		disconnect();
	});

	it('(c) scheduleReconnect fetches a fresh idToken per attempt', async () => {
		const { connect, disconnect } = await import('./ws');
		connect('initial-token');

		const ws = MockWebSocket.instances[0];
		ws.triggerOpen();

		// Close with normal code (1000) — triggers a reconnect attempt.
		ws.triggerClose(1000);

		// Advance timers past BASE_RECONNECT_DELAY_MS (1000ms).
		await vi.advanceTimersByTimeAsync(1500);
		await vi.runAllTimersAsync();

		// idToken should have been called for reconnect.
		expect(mockIdToken).toHaveBeenCalled();

		disconnect();
	});

	it('(d) after MAX_RECONNECT_ATTEMPTS the state stays disconnected', async () => {
		mockIdToken.mockResolvedValue('tok');
		const { connect, connectionState } = await import('./ws');

		let state = 'disconnected';
		connectionState.subscribe((s) => (state = s));

		connect('tok');
		MockWebSocket.instances[0].triggerOpen();

		// Exhaust all 10 reconnect attempts using normal close codes.
		// Each attempt doubles delay: 1s, 2s, 4s, 8s, 16s, 30s(cap)...
		for (let i = 0; i < 11; i++) {
			const ws = MockWebSocket.instances[MockWebSocket.instances.length - 1];
			ws.triggerClose(1000);
			await vi.advanceTimersByTimeAsync(35_000);
			await vi.runAllTimersAsync();
		}

		// After exhausting attempts, connectionState should be 'disconnected'.
		expect(state).toBe('disconnected');
	});

	it('(e) 1006 on first attempt triggers force-refresh; second 1006 within 60s sets authError', async () => {
		const { connect, disconnect } = await import('./ws');

		// First connect + 1006 on attempt 0 → force-refresh path (no authError yet).
		connect('tok');
		MockWebSocket.instances[0].triggerClose(1006);
		expect(mockAuthErrorSet).not.toHaveBeenCalled();

		// Advance timers to trigger the reconnect.
		await vi.advanceTimersByTimeAsync(1500);
		await vi.runAllTimersAsync();

		// New socket was opened; reconnectAttempt incremented to 1.
		// Now call connect() fresh to reset reconnectAttempt back to 0.
		connect('tok');
		// Immediately close with 1006 again — within 60s cooldown → authError.
		const latestWs =
			MockWebSocket.instances[MockWebSocket.instances.length - 1];
		latestWs.triggerClose(1006);

		expect(mockAuthErrorSet).toHaveBeenCalledWith(
			expect.objectContaining({ kind: 'auth_failed' })
		);

		disconnect();
	});

	it('(f) onMessage logs warn when overwriting an existing handler', async () => {
		const warnSpy = vi
			.spyOn(console, 'warn')
			.mockImplementation(() => undefined);

		const { onMessage, removeHandler } = await import('./ws');

		const handlerA = vi.fn();
		const handlerB = vi.fn();

		// Register first — no warn.
		onMessage(1, handlerA);
		expect(warnSpy).not.toHaveBeenCalled();

		// Register second for same type — should warn.
		onMessage(1, handlerB);
		expect(warnSpy).toHaveBeenCalledWith(
			expect.stringContaining('overwriting handler'),
			expect.anything()
		);

		removeHandler(1);
		warnSpy.mockRestore();
	});
});
