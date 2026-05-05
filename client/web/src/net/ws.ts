// WebSocket helper that opens the socket, runs the AUTH handshake from
// Phase 6 (first frame must be AUTH_REQUEST), and routes inbound envelopes
// to subscribers. Auto-reconnect refreshes the Firebase ID token to dodge
// the 1h expiry.

import { Build, MessageType, decodeEnvelope, type AuthResponseBody, type MessageTypeValue } from './proto';
import { auth } from '../auth/auth.svelte';

type Listener = (body: any, requestId: string) => void;

export type ConnState = 'idle' | 'connecting' | 'authenticating' | 'connected' | 'closed';

const reconnectBackoffMs = [500, 1000, 2000, 4000, 8000];

export class WsClient {
  private url: string;
  private ws: WebSocket | null = null;
  private listeners = new Map<MessageTypeValue, Set<Listener>>();
  private reconnectAttempts = 0;
  private intentionalClose = false;

  state = $state<ConnState>('idle');
  uid = $state<string | null>(null);
  lastError = $state<string | null>(null);

  constructor(url: string) {
    this.url = url;
  }

  on(type: MessageTypeValue, listener: Listener): () => void {
    let set = this.listeners.get(type);
    if (!set) {
      set = new Set();
      this.listeners.set(type, set);
    }
    set.add(listener);
    return () => set?.delete(listener);
  }

  send(buf: Uint8Array) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(buf);
    }
  }

  // Open + authenticate. Resolves on AUTH_RESPONSE{ok}, rejects on close
  // or AUTH_RESPONSE{ok:false}.
  async connect(): Promise<void> {
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
        ws.send(Build.auth(token));
      };

      ws.onmessage = (ev) => {
        const buf = new Uint8Array(ev.data as ArrayBuffer);
        const env = decodeEnvelope(buf);
        if (this.state === 'authenticating' && env.type === MessageType.AUTH_RESPONSE) {
          const ack = env.body as AuthResponseBody;
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
        } else if (this.state === 'connecting') {
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

  private scheduleReconnect() {
    const delay = reconnectBackoffMs[Math.min(this.reconnectAttempts, reconnectBackoffMs.length - 1)];
    this.reconnectAttempts += 1;
    setTimeout(() => {
      // Force a token refresh on reconnect to clear potential expiry.
      auth.getIdToken(true).then(() => this.connect().catch(() => this.scheduleReconnect()));
    }, delay);
  }
}

// Default factory wires the URL from env or relative `/ws`.
export function defaultWsUrl(): string {
  const env = import.meta.env.VITE_WS_URL as string | undefined;
  if (env) return env;
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
  return `${proto}//${location.host}/ws`;
}
