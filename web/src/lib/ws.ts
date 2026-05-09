// WebSocket client: binary protobuf envelope transport with request/response
// correlation, exponential-backoff reconnect, and 50-min token refresh.
// No `any` types — payload bytes are Uint8Array throughout; callers decode.
// Uses @bufbuild/protobuf v2 API: create/toBinary/fromBinary are module fns.
// See research report §4 for the sketch this is based on.
import { writable } from 'svelte/store';
import { create, toBinary, fromBinary } from '@bufbuild/protobuf';
import {
	EnvelopeSchema,
	AuthRefreshSchema,
	MessageType
} from './pb/dleague/v1/envelope_pb';
import type { Envelope } from './pb/dleague/v1/envelope_pb';
import { idToken } from './auth-store';

// ── Types ─────────────────────────────────────────────────────────────────────

export type ConnectionState = 'disconnected' | 'connecting' | 'connected';

type PendingRequest = {
	resolve: (payload: Uint8Array) => void;
	reject: (err: Error) => void;
	timeoutId: ReturnType<typeof setTimeout>;
};

type MessageHandler = (payload: Uint8Array, requestId: string) => void;

// ── Public stores ─────────────────────────────────────────────────────────────

/** Reactive connection state consumed by ConnectionStatus component. */
export const connectionState = writable<ConnectionState>('disconnected');

// ── Module state ──────────────────────────────────────────────────────────────

const MAX_RECONNECT_ATTEMPTS = 10;
const BASE_RECONNECT_DELAY_MS = 1000;
const MAX_RECONNECT_DELAY_MS = 30_000;
const REQUEST_TIMEOUT_MS = 5_000;
// Refresh 10 min before the 60-min Firebase token expiry (at 50 min).
const TOKEN_REFRESH_INTERVAL_MS = 50 * 60 * 1000;

const pending = new Map<string, PendingRequest>();
const handlers = new Map<MessageType, MessageHandler>();

let socket: WebSocket | null = null;
let reconnectAttempt = 0;
let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
let refreshTimer: ReturnType<typeof setTimeout> | null = null;
// Set true when disconnect() is called intentionally to prevent auto-reconnect.
let closed = false;

// ── Connection lifecycle ──────────────────────────────────────────────────────

/**
 * connect opens a WebSocket using the provided Firebase ID token for the
 * Sec-WebSocket-Protocol handshake. The server (Phase 05) expects:
 *   Sec-WebSocket-Protocol: dleague.v1, fb.<idToken>
 */
export function connect(token: string): void {
	closed = false;
	reconnectAttempt = 0;
	openSocket(token);
}

/** disconnect closes the socket permanently (no reconnect). */
export function disconnect(): void {
	closed = true;
	clearTimers();
	if (socket) {
		socket.close(1000, 'client disconnect');
		socket = null;
	}
	connectionState.set('disconnected');
}

function openSocket(token: string): void {
	if (socket && socket.readyState < WebSocket.CLOSING) {
		socket.close(1000, 'reconnect');
	}

	connectionState.set('connecting');

	// Relative URL works in both Vite dev proxy (:5173 → :8080) and production.
	const ws = new WebSocket('/ws', ['dleague.v1', `fb.${token}`]);
	ws.binaryType = 'arraybuffer';
	socket = ws;

	ws.onopen = () => {
		reconnectAttempt = 0;
		connectionState.set('connected');
		scheduleTokenRefresh();
	};

	ws.onmessage = (evt: MessageEvent<ArrayBuffer>) => {
		handleIncoming(new Uint8Array(evt.data));
	};

	ws.onerror = () => {
		// onerror is always followed by onclose; reconnect logic lives in onclose.
	};

	ws.onclose = (evt: CloseEvent) => {
		connectionState.set('disconnected');
		clearTokenRefresh();
		if (!closed && reconnectAttempt < MAX_RECONNECT_ATTEMPTS) {
			scheduleReconnect(token, evt.code);
		} else if (!closed && reconnectAttempt >= MAX_RECONNECT_ATTEMPTS) {
			rejectAllPending(new Error('WebSocket: max reconnection attempts exceeded'));
		}
	};
}

function scheduleReconnect(token: string, closeCode: number): void {
	// 1001 = going away (server restart) — reset counter for a clean reconnect.
	if (closeCode === 1001) reconnectAttempt = 0;
	const delay = Math.min(
		BASE_RECONNECT_DELAY_MS * Math.pow(2, reconnectAttempt),
		MAX_RECONNECT_DELAY_MS
	);
	reconnectAttempt++;
	reconnectTimer = setTimeout(() => openSocket(token), delay);
}

// ── Message dispatch ──────────────────────────────────────────────────────────

function handleIncoming(data: Uint8Array): void {
	let env: Envelope;
	try {
		env = fromBinary(EnvelopeSchema, data);
	} catch {
		console.error('ws: failed to decode Envelope');
		return;
	}

	// Resolve request/response correlation first.
	const p = pending.get(env.requestId);
	if (p) {
		clearTimeout(p.timeoutId);
		pending.delete(env.requestId);
		if (env.type === MessageType.ERROR) {
			p.reject(new Error(`server error for request ${env.requestId}`));
		} else {
			p.resolve(env.payload);
		}
	}

	// Always invoke a fire-and-forget handler too (server-push messages).
	const handler = handlers.get(env.type);
	if (handler) handler(env.payload, env.requestId);
}

// ── Public API ────────────────────────────────────────────────────────────────

/**
 * sendRequest sends a typed envelope and returns a Promise that resolves with
 * the raw payload bytes of the matching response envelope.
 * Uses crypto.randomUUID() for request_id (no external dep).
 */
export function sendRequest(
	type: MessageType,
	payload: Uint8Array,
	timeoutMs: number = REQUEST_TIMEOUT_MS
): Promise<Uint8Array> {
	return new Promise<Uint8Array>((resolve, reject) => {
		if (!socket || socket.readyState !== WebSocket.OPEN) {
			reject(new Error('WebSocket not connected'));
			return;
		}

		const requestId = crypto.randomUUID();
		const timeoutId = setTimeout(() => {
			pending.delete(requestId);
			reject(new Error(`request timeout (type=${type}, id=${requestId})`));
		}, timeoutMs);

		pending.set(requestId, { resolve, reject, timeoutId });

		const env = create(EnvelopeSchema, { type, requestId, payload });
		try {
			socket.send(toBinary(EnvelopeSchema, env));
		} catch (err) {
			clearTimeout(timeoutId);
			pending.delete(requestId);
			reject(err instanceof Error ? err : new Error(String(err)));
		}
	});
}

/**
 * onMessage registers a fire-and-forget handler for a specific MessageType.
 * One handler per type; later registrations overwrite earlier ones.
 */
export function onMessage(type: MessageType, handler: MessageHandler): void {
	handlers.set(type, handler);
}

/** removeHandler deregisters a previously registered handler. */
export function removeHandler(type: MessageType): void {
	handlers.delete(type);
}

// ── Token refresh ─────────────────────────────────────────────────────────────

/**
 * scheduleTokenRefresh sets a timer to rotate the Firebase ID token at 50 min.
 * Sends AUTH_REFRESH to the server (Phase 05 contract); server replies with
 * AUTH_REFRESH_ACK{expires_at_unix} and updates the connection's token.
 */
function scheduleTokenRefresh(): void {
	clearTokenRefresh();
	refreshTimer = setTimeout(async () => {
		try {
			const newToken = await idToken();
			if (!socket || socket.readyState !== WebSocket.OPEN) return;
			const refreshMsg = create(AuthRefreshSchema, { idToken: newToken });
			const refreshPayload = toBinary(AuthRefreshSchema, refreshMsg);
			const env = create(EnvelopeSchema, {
				type: MessageType.AUTH_REFRESH,
				requestId: crypto.randomUUID(),
				payload: refreshPayload
			});
			socket.send(toBinary(EnvelopeSchema, env));
			// Chain: schedule next refresh after this one.
			scheduleTokenRefresh();
		} catch (err) {
			console.error('ws: token refresh failed', err);
		}
	}, TOKEN_REFRESH_INTERVAL_MS);
}

function clearTokenRefresh(): void {
	if (refreshTimer !== null) {
		clearTimeout(refreshTimer);
		refreshTimer = null;
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function clearTimers(): void {
	if (reconnectTimer !== null) {
		clearTimeout(reconnectTimer);
		reconnectTimer = null;
	}
	clearTokenRefresh();
}

function rejectAllPending(err: Error): void {
	for (const [id, p] of pending) {
		clearTimeout(p.timeoutId);
		p.reject(err);
		pending.delete(id);
	}
}
