// WebSocket helper that opens the socket, runs the AUTH handshake from
// Phase 6 (first frame must be AUTH_REQUEST), and routes inbound envelopes
// to subscribers. Auto-reconnect refreshes the Firebase ID token to dodge
// the 1h expiry.

import { Build, MessageType, decodeEnvelope } from './proto';
import { auth } from '../auth/auth.svelte';

/** @typedef {import('./proto').AuthResponseBody} AuthResponseBody */
/** @typedef {import('./proto').MessageTypeValue} MessageTypeValue */

/** @typedef {(body: any, requestId: string) => void} Listener */

/** @typedef {'idle' | 'connecting' | 'authenticating' | 'connected' | 'closed'} ConnState */

const reconnectBackoffMs = [500, 1000, 2000, 4000, 8000];

export class WsClient {
  /** @type {string} */
  url;
  /** @type {WebSocket | null} */
  ws = null;
  /** @type {Map<MessageTypeValue, Set<Listener>>} */
  listeners = new Map();
  /** @type {number} */
  reconnectAttempts = 0;
  /** @type {boolean} */
  intentionalClose = false;

  state = $state(/** @type {ConnState} */ ('idle'));
  uid = $state(/** @type {string | null} */ (null));
  lastError = $state(/** @type {string | null} */ (null));

  /** @param {string} url */
  constructor(url) {
    this.url = url;
  }

  /**
   * @param {MessageTypeValue} type
   * @param {Listener} listener
   * @returns {() => void}
   */
  on(type, listener) {
    let set = this.listeners.get(type);
    if (!set) {
      set = new Set();
      this.listeners.set(type, set);
    }
    set.add(listener);
    return () => set?.delete(listener);
  }

  /** @param {Uint8Array} buf */
  send(buf) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(/** @type {ArrayBufferView<ArrayBuffer>} */ (buf));
    }
  }

  // Open + authenticate. Resolves on AUTH_RESPONSE{ok}, rejects on close
  // or AUTH_RESPONSE{ok:false}.
  /** @returns {Promise<void>} */
  async connect() {
    this.intentionalClose = false;
    const token = await auth.getIdToken(false);
    if (!token) throw new Error('not signed in');

    return new Promise((resolve, reject) => {
      this.state = 'connecting';
      const ws = new WebSocket(this.url);
      ws.binaryType = 'arraybuffer';
      this.ws = ws;

      ws.onopen = () => {
        this.state = 'authenticating';
        ws.send(/** @type {ArrayBufferView<ArrayBuffer>} */ (Build.auth(token)));
      };

      ws.onmessage = (ev) => {
        const buf = new Uint8Array(/** @type {ArrayBuffer} */ (ev.data));
        const env = decodeEnvelope(buf);
        if (this.state === 'authenticating' && env.type === MessageType.AUTH_RESPONSE) {
          const ack = /** @type {AuthResponseBody} */ (env.body);
          if (!ack.ok) {
            this.lastError = ack.error || 'auth rejected';
            this.state = 'closed';
            ws.close();
            reject(new Error(this.lastError));
            return;
          }
          this.uid = ack.uid;
          this.state = 'connected';
          this.reconnectAttempts = 0;
          resolve();
          return;
        }
        const set = this.listeners.get(env.type);
        if (set) for (const fn of set) fn(env.body, env.requestId);
      };

      ws.onerror = () => {
        this.lastError = 'socket error';
      };

      ws.onclose = () => {
        const wasConnected = this.state === 'connected';
        this.state = 'closed';
        this.ws = null;
        if (this.intentionalClose) return;
        if (wasConnected) {
          this.scheduleReconnect();
        } else if (/** @type {any} */ (this.state) === 'connecting') {
          // Initial connect failed before resolving.
          reject(new Error('connect failed'));
        }
      };
    });
  }

  close() {
    this.intentionalClose = true;
    this.ws?.close();
  }

  scheduleReconnect() {
    const delay = reconnectBackoffMs[Math.min(this.reconnectAttempts, reconnectBackoffMs.length - 1)];
    this.reconnectAttempts += 1;
    setTimeout(() => {
      // Force a token refresh on reconnect to clear potential expiry.
      auth.getIdToken(true).then(() => this.connect().catch(() => this.scheduleReconnect()));
    }, delay);
  }
}

// Default factory wires the URL from env or relative `/ws`.
/** @returns {string} */
export function defaultWsUrl() {
  const env = /** @type {string | undefined} */ (import.meta.env.VITE_WS_URL);
  if (env) return env;
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
  return `${proto}//${location.host}/ws`;
}
