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
import {
	QueueJoinSchema,
	QueueMatchedSchema,
	MatchMoveSchema,
	MatchOpponentProgressSchema,
	MatchResolvedSchema,
	MatchRejoinSchema,
	MatchRejoinAckSchema
} from './pb/dleague/v1/match_pb';
import type {
	QueueMatched,
	MatchOpponentProgress,
	MatchResolved,
	MatchRejoinAck
} from './pb/dleague/v1/match_pb';
import { idToken } from './auth-store';
import { authError } from './auth-error-store';

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
// Auth-reject force-refresh state: tracks last force-refresh to cap at 1/min.
let lastForceRefreshAt = 0;
const FORCE_REFRESH_COOLDOWN_MS = 60_000;
let pendingForceRefresh = false;

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
		rejectAllPending(new Error('WebSocket: connection lost'));

		// Code 1006 = abnormal closure, which is what browsers report when the
		// server rejects the HTTP upgrade (our auth failure path: HTTP 401 before
		// WS handshake completes). Codes 1008/4401 kept for forward-compat.
		const isAuthReject = evt.code === 1006 || evt.code === 1008 || evt.code === 4401;
		if (isAuthReject && reconnectAttempt === 0) {
			// Only treat first-attempt 1006 as auth reject; subsequent 1006s
			// may be network drops, handled by normal backoff.
			const now = Date.now();
			if (now - lastForceRefreshAt > FORCE_REFRESH_COOLDOWN_MS) {
				lastForceRefreshAt = now;
				pendingForceRefresh = true;
			} else {
				authError.set({ kind: 'auth_failed', message: 'Authentication failed; please sign in again' });
				return;
			}
		}

		if (!closed && reconnectAttempt < MAX_RECONNECT_ATTEMPTS) {
			scheduleReconnect(evt.code);
		}
	};
}

function scheduleReconnect(closeCode: number): void {
	if (reconnectAttempt >= MAX_RECONNECT_ATTEMPTS || closed) {
		connectionState.set('disconnected');
		return;
	}
	// 1001 = going away (server restart) — reset counter for a clean reconnect.
	if (closeCode === 1001) reconnectAttempt = 0;
	const delay = Math.min(
		BASE_RECONNECT_DELAY_MS * Math.pow(2, reconnectAttempt),
		MAX_RECONNECT_DELAY_MS
	);
	reconnectAttempt++;
	// Fetch a fresh token at each retry — the old one may have expired.
	reconnectTimer = setTimeout(async () => {
		try {
			const force = pendingForceRefresh;
			pendingForceRefresh = false;
			const fresh = await idToken(force);
			openSocket(fresh);
		} catch {
			scheduleReconnect(0);
		}
	}, delay);
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

// ── Phase 09: sync PvP helpers ────────────────────────────────────────────────

/**
 * sendQueueJoin sends a QUEUE_JOIN message to enter the matchmaking queue.
 * Fire-and-forget: server pushes QUEUE_MATCHED when a pair is found.
 */
export function sendQueueJoin(gameId: string): void {
	if (!socket || socket.readyState !== WebSocket.OPEN) return;
	const payload = toBinary(QueueJoinSchema, create(QueueJoinSchema, { gameId }));
	const env = create(EnvelopeSchema, {
		type: MessageType.QUEUE_JOIN,
		requestId: '',
		payload
	});
	socket.send(toBinary(EnvelopeSchema, env));
}

/**
 * sendQueueLeave sends a QUEUE_LEAVE message to exit the matchmaking queue.
 * Fire-and-forget: no response expected.
 */
export function sendQueueLeave(): void {
	if (!socket || socket.readyState !== WebSocket.OPEN) return;
	const env = create(EnvelopeSchema, {
		type: MessageType.QUEUE_LEAVE,
		requestId: '',
		payload: new Uint8Array(0)
	});
	socket.send(toBinary(EnvelopeSchema, env));
}

/**
 * sendMatchMove sends a MATCH_MOVE message with the player's guess.
 * The server will push back a WORDLE_STATE envelope for own state, and
 * MATCH_OPPONENT_PROGRESS to the opponent.
 */
export function sendMatchMove(matchId: string, guess: string): void {
	if (!socket || socket.readyState !== WebSocket.OPEN) return;
	const payload = toBinary(MatchMoveSchema, create(MatchMoveSchema, { matchId, guess }));
	const env = create(EnvelopeSchema, {
		type: MessageType.MATCH_MOVE,
		requestId: crypto.randomUUID(),
		payload
	});
	socket.send(toBinary(EnvelopeSchema, env));
}

/**
 * sendMatchRejoin sends a MATCH_REJOIN to reclaim an interrupted session.
 * Returns a Promise that resolves with the MatchRejoinAck payload bytes.
 */
export function sendMatchRejoin(matchId: string): Promise<MatchRejoinAck> {
	const payload = toBinary(MatchRejoinSchema, create(MatchRejoinSchema, { matchId }));
	return sendRequest(MessageType.MATCH_REJOIN, payload).then((bytes) =>
		fromBinary(MatchRejoinAckSchema, bytes)
	);
}

/**
 * onQueueMatched registers a handler for the server-pushed QUEUE_MATCHED message.
 */
export function onQueueMatched(handler: (msg: QueueMatched) => void): void {
	onMessage(MessageType.QUEUE_MATCHED, (payload) => {
		handler(fromBinary(QueueMatchedSchema, payload));
	});
}

/**
 * onMatchOpponentProgress registers a handler for opponent progress pushes.
 */
export function onMatchOpponentProgress(handler: (msg: MatchOpponentProgress) => void): void {
	onMessage(MessageType.MATCH_OPPONENT_PROGRESS, (payload) => {
		handler(fromBinary(MatchOpponentProgressSchema, payload));
	});
}

/**
 * onMatchResolved registers a handler for match resolution pushes.
 */
export function onMatchResolved(handler: (msg: MatchResolved) => void): void {
	onMessage(MessageType.MATCH_RESOLVED, (payload) => {
		handler(fromBinary(MatchResolvedSchema, payload));
	});
}

/**
 * onMatchRejoinAck registers a handler for MATCH_REJOIN_ACK server pushes.
 * (Also handled via sendMatchRejoin's Promise; register here for layout-level handling.)
 */
export function onMatchRejoinAck(handler: (msg: MatchRejoinAck) => void): void {
	onMessage(MessageType.MATCH_REJOIN_ACK, (payload) => {
		handler(fromBinary(MatchRejoinAckSchema, payload));
	});
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
