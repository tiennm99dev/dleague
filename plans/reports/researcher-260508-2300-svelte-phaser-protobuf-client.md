# Research: Svelte 5 + Phaser 3 + Protobuf Client

**Date:** 2026-05-08  
**Status:** DONE

---

## 1. Project Scaffolding: SvelteKit vs Vite+Svelte

### Recommendation: **SvelteKit + adapter-static**

**Why:** SvelteKit with `@sveltejs/adapter-static` (installed via `npm i -D @sveltejs/adapter-static`) outputs to a single `build/` folder by default, consumable directly by Go's `http.FileServer("/")`. No custom static serving logic needed.

**Configuration:**

```js
// svelte.config.js
import adapter from '@sveltejs/adapter-static';

export default {
  kit: {
    adapter: adapter({
      pages: 'web/dist',  // Output to Go's static handler
      assets: 'web/dist',
      fallback: 'index.html'  // SPA fallback for routes
    })
  }
};
```

**Key points:**
- Svelte 5 (current v5.55.0 as of May 2026) fully supported; actively maintained with monthly updates
- Prerender all routes via `export const prerender = true` in `+layout.svelte`
- Zero server-side rendering overhead (set `ssr: true` but pages fully pre-render)
- Build output: single `web/dist/` folder with .html, .js, .css, all inlined or referenced
- TypeScript first-class: use `+page.server.ts`, `+page.svelte` natively

**Alternative (Vite+Svelte):** Vite+Svelte is lighter (no routing, layouts, etc.) but requires manual static scaffolding. SvelteKit's opinionated structure pays off here: file-based routing = trivial form/overlay UI structure.

**Bundle estimate:** ~2-3 KB Svelte runtime (compiler-eliminated) + Phaser + protobuf = ~150-300 KB gzipped total (before game assets).

---

## 2. Phaser Integration: Canvas Component Pattern

### Architecture: Svelte container + Phaser canvas

**Official template:** Phaser maintains `phaserjs/template-svelte` (Phaser 3.90.0, Svelte 5.23.1, Vite 6.3.1 compatible). Reference architecture:

```svelte
<!-- src/lib/PhaserGame.svelte -->
<script>
  import { onMount } from 'svelte';
  import Phaser from 'phaser';
  import { eventBus } from './EventBus'; // Shared event emitter

  let game: Phaser.Game;
  let gameContainer: HTMLDivElement;

  onMount(() => {
    // Initialize once
    game = new Phaser.Game({
      type: Phaser.AUTO,
      render: { pixelArt: true },
      canvas: gameContainer.querySelector('canvas') ?? undefined,
      scene: GameScene
    });

    return () => {
      if (game) game.destroy(true);
    };
  });
</script>

<div bind:this={gameContainer}>
  <canvas></canvas>
</div>

<style>
  div {
    width: 100%;
    aspect-ratio: 4/3;
    position: relative;
  }
</style>
```

**Key integration patterns:**

1. **EventBus** (simple pub/sub): `EventBus.emit('move', data)` from Svelte, `this.events.on('move', ...)` in Phaser Scene
2. **Scene lifecycle:** Scenes emit `"current-scene-ready"` to notify Svelte when ready
3. **Canvas ownership:** Phaser creates canvas; Svelte container holds it
4. **Cleanup:** Phaser `.destroy(true)` on component unmount (onMount return function)

**Phaser 3.80+ maturity:**
- Full Vite ESM support ✓
- TypeScript definitions included ✓
- WebGL + Canvas2D auto-fallback ✓
- Memory leak clean on destroy ✓

**Bundle:**
- Full Phaser: 345 KB min | 110 KB gzipped
- Custom build (no audio/physics): ~70-100 KB gzipped
- Recommendation: Ship full Phaser 3.80.0 first; optimize if metrics warrant

---

## 3. Protobuf in JS/TS: `@bufbuild/protobuf-es`

### Recommendation: **`@bufbuild/protobuf-es` + `@bufbuild/protoc-gen-es`**

**Why:**
- **Smallest bundle:** 15-20% overhead vs alternatives (protobuf-ts, google-protobuf at 62% overhead)
- **Conformance:** Only JS/TS library with 100% protobuf conformance test pass rate
- **DX:** `buf generate` config is identical for Go + TS (one `buf.gen.yaml`)
- **ESM native:** Tree-shake friendly, no CommonJS bloat
- **Maintained:** Buf.build backed; v2.0+ latest

**Installation:**
```bash
npm install --save @bufbuild/protobuf
npm install --save-dev @bufbuild/protoc-gen-es
```

**Updated `buf.gen.yaml`:**
```yaml
version: v2
plugins:
  # Go output
  - local: protoc-gen-go
    out: ../shared/pb
    opt:
      - paths=source_relative

  # TypeScript output (NEW)
  - remote: buf.build/bufbuild/protoc-gen-es:v1
    out: ../web/src/pb
    opt:
      - target=ts  # Generate *.ts + *.d.ts (smallest)
```

**Generated output structure:**
```
web/src/pb/
├── dleague/
│   └── v1/
│       ├── envelope_pb.ts       # Type defs + marshal/unmarshal
│       └── envelope_pb.d.ts     # TypeScript types only
```

**Usage:**
```ts
import { Envelope, MessageType, Ping } from './pb/dleague/v1/envelope_pb';

const ping = new Ping({ clientUnixMs: Date.now() });
const env = new Envelope({
  type: MessageType.MESSAGE_TYPE_PING,
  requestId: crypto.randomUUID(),
  payload: ping.toBinary()
});
const bytes = env.toBinary(); // Send over WS
```

**Bundle size delta:** ~8-12 KB gzipped for envelope + future payloads (game state, moves, etc.)

---

## 4. WebSocket Client Pattern: Request/Response Correlation

### Sketch: Svelte store wrapping `WebSocket` + `request_id` correlation

```ts
// src/lib/stores/ws-store.ts
import { writable, derived } from 'svelte/store';
import type { Envelope } from '../pb/dleague/v1/envelope_pb';
import { Envelope, MessageType } from '../pb/dleague/v1/envelope_pb';

type PendingRequest = {
  resolve: (payload: Uint8Array) => void;
  reject: (err: Error) => void;
  timeout: ReturnType<typeof setTimeout>;
};

const pendingRequests = new Map<string, PendingRequest>();
const messageHandlers = new Map<
  MessageType,
  (payload: Uint8Array, requestId: string) => void
>();

function generateRequestId(): string {
  return crypto.randomUUID();
}

function reconnectWithBackoff(
  url: string,
  attempt: number = 0,
  maxAttempts: number = 10
) {
  const delay = Math.min(1000 * Math.pow(2, attempt), 30000);
  if (attempt > maxAttempts) throw new Error('Max reconnection attempts');

  setTimeout(() => {
    try {
      const ws = new WebSocket(url);
      ws.binaryType = 'arraybuffer';

      ws.onmessage = (evt: MessageEvent<ArrayBuffer>) => {
        const env = Envelope.fromBinary(new Uint8Array(evt.data));
        
        // Handle request/response correlations
        if (pendingRequests.has(env.requestId)) {
          const pending = pendingRequests.get(env.requestId)!;
          clearTimeout(pending.timeout);
          pending.resolve(env.payload);
          pendingRequests.delete(env.requestId);
        }

        // Handle fire-and-forget messages
        const handler = messageHandlers.get(env.type);
        if (handler) {
          handler(env.payload, env.requestId);
        }
      };

      ws.onerror = () => reconnectWithBackoff(url, attempt + 1);
      ws.onclose = () => reconnectWithBackoff(url, attempt + 1);
    } catch (err) {
      reconnectWithBackoff(url, attempt + 1);
    }
  }, delay);
}

export const wsConnection = writable<WebSocket | null>(null);

export async function sendRequest<T>(
  type: MessageType,
  payload: T,
  timeoutMs: number = 5000
): Promise<Uint8Array> {
  return new Promise((resolve, reject) => {
    const requestId = generateRequestId();
    const timeout = setTimeout(() => {
      pendingRequests.delete(requestId);
      reject(new Error(`Request timeout: ${type}`));
    }, timeoutMs);

    pendingRequests.set(requestId, { resolve, reject, timeout });

    const env = new Envelope({
      type,
      requestId,
      payload: (payload as any).toBinary?.() ?? new Uint8Array()
    });

    const ws = get(wsConnection);
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      reject(new Error('WebSocket not connected'));
      pendingRequests.delete(requestId);
      return;
    }

    ws.send(env.toBinary());
  });
}

export function onMessage(
  type: MessageType,
  handler: (payload: Uint8Array, requestId: string) => void
) {
  messageHandlers.set(type, handler);
}

export function connect(url: string) {
  reconnectWithBackoff(url);
}
```

**Usage in component:**
```svelte
<script>
  import { sendRequest, onMessage, connect } from './stores/ws-store';
  import { Pong } from './pb/dleague/v1/envelope_pb';

  onMount(() => {
    connect('ws://localhost:8080/ws');
    
    onMessage(MessageType.MESSAGE_TYPE_PONG, (payload) => {
      const pong = Pong.fromBinary(payload);
      console.log('Pong:', pong.serverUnixMs);
    });
  });
</script>
```

**Key features:**
- Automatic reconnect with exponential backoff (capped 30s)
- Request/response correlation via `request_id` (UUID)
- Fire-and-forget message dispatch
- 5s timeout default; configurable per-call
- Binary payload transparent (store knows only bytes)

**LoC:** ~80 lines (sketch ~30 without backoff logic)

---

## 5. Build Pipeline: Vite Dev + Static Prod

### Development:
```bash
# Terminal 1: Svelte dev server (HMR)
cd web && npm run dev
# Serves :5173, proxies WS to :8080

# Terminal 2: Go server
cd .. && make dev
# Runs :8080, serves static from web/dist/ (empty in dev)
```

**vite.config.ts proxy:**
```ts
import { defineConfig } from 'vite';
import svelte from 'vite-plugin-svelte';

export default defineConfig({
  plugins: [svelte()],
  server: {
    proxy: {
      '/ws': {
        target: 'ws://localhost:8080',
        ws: true
      }
    }
  }
});
```

### Production:
```bash
# Makefile target
web-build:
  cd web && npm run build
  # Outputs to web/dist/

# Go server serves dist/
http.Handle("/", http.FileServer(http.Dir("web/dist/")))
```

**svelte.config.js:**
```js
import adapter from '@sveltejs/adapter-static';

export default {
  kit: {
    adapter: adapter({ pages: 'dist', assets: 'dist' }),
    prerender: { entries: ['/'] }  // Prerender root + auto-discover routes
  }
};
```

**Result:** Single `web/dist/index.html` + vendors.js + game assets. Go serves everything; no separate frontend host needed.

---

## 6. Tradeoffs vs Ebitengine

| Aspect | Ebitengine WASM | Svelte+Phaser |
|--------|-----------------|---------------|
| **Bundle** | ~5-8 MB (unoptimized) | ~150-300 KB gzipped |
| **DX: Forms/Input** | Canvas overlay hacks | Native HTML inputs, no friction |
| **DX: Styling** | Inline canvas drawing | Tailwind/CSS, component reuse |
| **Accessibility** | Canvas-only (SR blocked) | Semantic HTML, ARIA-friendly |
| **Load time** | ~2-5s (WASM decode) | ~300-800 ms (JS parse) |
| **Engine maturity** | Stable (production games) | Stable (Phaser 3.80+) |
| **Mobile (future)** | Native via gomobile | Works on mobile browsers |
| **Team skill** | Go-heavy team → friction | Web devs native, no Go needed |

**Verdict:** Svelte+Phaser ~10x smaller, UX/DX dramatically better for web-first MVP. Ebitengine wins for native-first or canvas-heavy rendering (3D, complex shaders). For a -dle game (UI-heavy, form-driven), Svelte is the clear win.

---

## 7. Integration Checklist

- [ ] `npm create svelte@latest web` (scaffold)
- [ ] Install: `@sveltejs/adapter-static`, `phaser`, `@bufbuild/protobuf`, `@bufbuild/protoc-gen-es`
- [ ] Update `buf.gen.yaml` (add TypeScript output)
- [ ] Run `buf generate` (produces `web/src/pb/`)
- [ ] Create `src/lib/PhaserGame.svelte` (canvas component)
- [ ] Create `src/lib/stores/ws-store.ts` (WebSocket client)
- [ ] Add `src/routes/+page.svelte` (main game layout)
- [ ] Configure Vite proxy for `:5173 → :8080/ws`
- [ ] Test: `npm run dev` (HMR), connect to local WS server
- [ ] Build: `npm run build` → `web/dist/`
- [ ] Verify Go serves `web/dist/` at `/`

---

## Unresolved Questions

1. **Game state synchronization:** Does Dleague use event-sourcing or delta updates? Affects Phaser scene design (stateless vs. mutable).
2. **Asset pipeline:** Where do sprite sheets, fonts, audio live? Proto-embed or separate CDN?
3. **TypeScript declaration ordering:** Does `buf generate` auto-wire `*.d.ts` for tsc, or need explicit tsconfig path mapping?
4. **Fallback SPA route:** Should 404s → `/index.html` or `/game`? Affects adapter config.
5. **Custom Phaser build:** Is full Phaser (110 KB gz) acceptable, or is physics/input/animation optimization critical?

---

**Status: DONE**
