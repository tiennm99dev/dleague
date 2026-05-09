---
phase: 6
title: "Svelte+Phaser client scaffold (replaces Ebitengine)"
status: completed
completed_on: 2026-05-09
priority: P1
effort: 1.5w
dependencies: [5]
---

# Phase 06 — Svelte+Phaser client scaffold

## Context Links
- `plans/reports/researcher-260508-2300-svelte-phaser-protobuf-client.md` (full integration guide)
- Existing client to delete: `client/` (Ebitengine WASM)
- Existing static to delete: `web/wasm_exec.js`, `web/index.html`
- `proto/buf.gen.yaml` (extend with TS plugin)
- `server/internal/http/router.go` (static FileServer + SPA fallback per code-review M9)

## Overview
Delete the entire Ebitengine module (`client/`) and WASM static shell. Create new `web/` SvelteKit + adapter-static + Phaser 3.80+ project. Generate TS protobuf alongside Go via `@bufbuild/protoc-gen-es`. Implement: WS client store with binary protobuf + request_id correlation + reconnect, Firebase JS SDK auth (Email/Pw + Google + Anonymous), Phaser canvas component with EventBus pattern, title scene replicating Ebitengine version. Server adds SPA-fallback handler so client-routed paths return `index.html`.

## Key Insights
- SvelteKit + adapter-static outputs single `web/dist/` consumable by Go FileServer (svelte-phaser report §1).
- Phaser canvas owned by Svelte container via `bind:this` + EventBus pub/sub (svelte-phaser report §2). Official template `phaserjs/template-svelte`.
- `@bufbuild/protobuf` + `protoc-gen-es` smallest TS bundle (svelte-phaser report §3); same `buf.gen.yaml` adds TS output.
- WS client: native `WebSocket` + `binaryType='arraybuffer'`; `Sec-WebSocket-Protocol` carries `dleague.v1` + `fb.<idToken>` (per Phase 05 contract).
- Firebase JS SDK auth-state subscription drives WS connect lifecycle: sign in → get token → connect; sign out → close.
- Vite dev: proxy `/ws` → `:8080` for ws+http (svelte-phaser report §5).
- Code-review M9 (router.go): SPA fallback — wildcard FileServer must return `index.html` on miss for client-side routing (`/match/<token>`).
- CSP `wasm-unsafe-eval` removed (Phase 02 added it as transitional).

## Requirements
**Functional:**
- New `web/` project (SvelteKit + TypeScript + adapter-static).
- Phaser 3.80+ canvas component with scene lifecycle + EventBus.
- TS protobuf generation via `buf generate` into `web/src/lib/pb/dleague/v1/`.
- WS client (`web/src/lib/ws.ts`, ~100 LOC): native WebSocket, binary protobuf encode/decode, request_id correlation (UUID v4), exponential backoff reconnect (cap 30s), auth via `Sec-WebSocket-Protocol`.
- Firebase JS SDK (`web/src/lib/firebase.ts`): init from env, expose `auth` + sign-in helpers (email/password, Google popup, anonymous).
- Auth-state-driven WS: on signed-in user, fetch ID token, open WS; on sign-out, close.
- Title scene migrated to Phaser (replicating `client/internal/scene/title.go` layout: dleague title + start button).
- `make web-build` runs `npm --prefix web run build`; outputs to `web/dist/`.
- Go server `router.go` adds SPA-fallback: handler reads `web/dist/index.html` for any non-asset, non-`/ws`, non-`/health` path that 404s in FileServer.
- Vite dev proxy `/ws` and `/health` to `localhost:8080`.
- CSP tightens (Phase 02 transitional `wasm-unsafe-eval` removed).

**Non-functional:**
- Bundle size: Svelte+Phaser+protobuf+firebase <400 KB gzipped. Measured via Vite `rollup-plugin-visualizer` once.
- Each Svelte component <200 LOC.
- TS strict mode on.
- No `any` in `ws.ts`.
- ES2022 target.

## Architecture
```
web/
├── package.json, tsconfig.json, svelte.config.js, vite.config.ts
├── firebase.config.json (commited; public web config)
├── src/
│   ├── app.html
│   ├── routes/
│   │   ├── +layout.svelte    (Firebase init, auth state subscription)
│   │   ├── +page.svelte      (title scene mount)
│   │   └── +page.ts          (export const prerender = true)
│   ├── lib/
│   │   ├── firebase.ts       (initializeApp + signInXxx helpers)
│   │   ├── auth-store.ts     (Svelte writable<User|null>)
│   │   ├── ws.ts             (WS client + req-resp correlation)
│   │   ├── pb/dleague/v1/    (generated)
│   │   ├── phaser/
│   │   │   ├── PhaserGame.svelte  (canvas mount + cleanup)
│   │   │   ├── event-bus.ts       (mitt or simple emitter)
│   │   │   └── scenes/
│   │   │       └── title-scene.ts
│   │   └── components/
│   │       ├── SignIn.svelte
│   │       └── ConnectionStatus.svelte
│   └── static/ (favicon, fonts)
└── dist/  (build output)
```

Data flow:
```
+layout.svelte: onMount → firebase.initializeApp → onAuthStateChanged(user)
  user ≠ null → user.getIdToken() → ws.connect(idToken)
  user == null → ws.close()

+page.svelte: <PhaserGame bind:scene={titleScene}/>
  TitleScene init → on "start" emit eventBus.emit('start-game')
+page.svelte listens eventBus → routes via goto('/play')
```

WS frame round-trip:
```
caller: sendRequest(MessageType.X, payload)
  ↳ generates UUID, builds Envelope, ws.send(env.toBinary())
  ↳ Promise pending; resolves when matching request_id arrives
ws.onmessage: Envelope.fromBinary(arraybuffer)
  ↳ if pending[req_id] → resolve
  ↳ else → handlers[type](payload)
```

## Related Code Files
**Create (web/):**
- `web/package.json`, `web/tsconfig.json`, `web/svelte.config.js`, `web/vite.config.ts`
- `web/firebase.config.json` (apiKey/authDomain/projectId — public web config; safe to commit)
- `web/src/app.html`, `web/src/app.d.ts`
- `web/src/routes/+layout.svelte`, `web/src/routes/+layout.ts`
- `web/src/routes/+page.svelte`, `web/src/routes/+page.ts`
- `web/src/lib/firebase.ts`
- `web/src/lib/auth-store.ts`
- `web/src/lib/ws.ts`
- `web/src/lib/phaser/phaser-game.svelte`
- `web/src/lib/phaser/event-bus.ts`
- `web/src/lib/phaser/scenes/title-scene.ts`
- `web/src/lib/components/sign-in.svelte`
- `web/src/lib/components/connection-status.svelte`
- `web/src/lib/components/auth-gate.svelte` (renders SignIn or slot)
- `web/.env.example`
- `web/.gitignore` (node_modules, dist, .svelte-kit)
- `server/internal/http/spa_fallback.go` — middleware: if request not for `/ws`, `/health`, or static file existing in dist, serve `index.html`.

**Modify:**
- `proto/buf.gen.yaml` — add TS plugin emitting to `web/src/lib/pb/`
- `Makefile` — add `web-install`, `web-build`, `web-dev` targets; `proto-gen` runs both Go + TS
- `server/internal/http/router.go` — wire SPA fallback for static; tighten CSP (drop `wasm-unsafe-eval`)
- `.github/workflows/ci.yml` — add Node setup + `npm ci` + `npm run build` step
- `.gitignore` — append `web/dist/`, `web/node_modules/`, `web/.svelte-kit/`
- `docs/system-architecture.md` — fill client section
- `docs/code-standards.md` — TypeScript conventions, kebab-case file naming for `.ts/.svelte`, no `any`

**Delete:**
- `client/` entire dir (cmd, internal, go.mod, go.sum)
- `web/wasm_exec.js`
- `web/index.html` (regenerated by SvelteKit)
- `go.work` entry for `client/`
- `client` references in CI / Makefile

## Implementation Steps
1. **Delete Ebitengine client:** `git rm -r client/ web/wasm_exec.js web/index.html`. Update `go.work` (remove `./client`). `go work sync`.
2. **Scaffold SvelteKit:** `npx sv create web --template minimal --types ts`. Choose adapter-static when prompted; install with `npm i -D @sveltejs/adapter-static`.
3. **Install runtime deps:** `cd web && npm i phaser firebase @bufbuild/protobuf && npm i -D @bufbuild/protoc-gen-es`.
4. **Configure adapter-static:** `web/svelte.config.js` per research §1, output to `web/dist/`, `fallback: 'index.html'` for SPA mode, prerender root.
5. **Vite proxy:** `web/vite.config.ts` proxies `/ws` (with `ws:true`) and `/health` to `localhost:8080`.
6. **Buf TS plugin:** edit `proto/buf.gen.yaml` to add second plugin entry: `remote: buf.build/bufbuild/protoc-gen-es:v1`, `out: ../web/src/lib/pb`, `opt: [target=ts]`. Run `buf generate` → verify TS files in `web/src/lib/pb/dleague/v1/envelope_pb.ts`.
7. **Firebase JS init:** `web/src/lib/firebase.ts`:
   - `initializeApp(firebaseConfig)`; `auth = getAuth()`.
   - If `import.meta.env.DEV`: `connectAuthEmulator(auth, "http://127.0.0.1:9099")`.
   - Export sign-in helpers: `signInWithEmail`, `signInWithGoogle` (popup), `signInAnon`.
8. **Auth store:** `web/src/lib/auth-store.ts` — `writable<User|null>`; subscribe to `onAuthStateChanged(auth, set)`. Export `idToken()` async helper that calls `currentUser.getIdToken()`.
9. **WS client (`web/src/lib/ws.ts`):**
   - Module state: `Map<string, PendingRequest>`, `Map<MessageType, Handler>`.
   - `connect(idToken: string)`: `new WebSocket('/ws', ['dleague.v1', \`fb.${idToken}\`])`; `binaryType='arraybuffer'`.
   - `onmessage`: `Envelope.fromBinary(new Uint8Array(evt.data))` → resolve pending or invoke handler.
   - `sendRequest<T>(type, payload, timeoutMs=5000)`: builds Envelope, awaits matching response.
   - `onMessage(type, handler)`: registers fire-and-forget handler.
   - Reconnect: exponential backoff capped 30s; max 10 attempts; emits `connection-state` to Svelte store.
   - **Token refresh:** every 50 min, fetch new idToken; send `MESSAGE_TYPE_AUTH_REFRESH{idToken}` (Phase 05 contract); on `AUTH_REFRESH_ACK` update internal expiry.
10. **PhaserGame component:** `web/src/lib/phaser/phaser-game.svelte` — `onMount` instantiates `new Phaser.Game({type: AUTO, scene: [TitleScene], parent: container})`; cleanup `game.destroy(true)` on unmount. Uses `eventBus` for scene→Svelte communication.
11. **Title scene:** `title-scene.ts` — replicate Ebitengine `client/internal/scene/title.go` layout (Dleague title + Start button). On click, `eventBus.emit('title:start')`. Tile sizes from research §2.
12. **Routes:**
    - `+layout.svelte`: mount Firebase auth state subscription; show SignIn if user null, else slot.
    - `+page.svelte`: `<PhaserGame/>`; listen for `title:start` → goto `/play` (Phase 07 fills).
13. **SignIn component:** `sign-in.svelte` — three buttons (Email, Google, Anonymous); calls helpers from `firebase.ts`.
14. **Connection status:** small badge top-right showing WS state (connecting / connected / disconnected).
15. **Server SPA fallback:** `spa_fallback.go` — middleware: if request method GET, path doesn't start with `/ws` or `/health`, and target file doesn't exist in dist, serve `web/dist/index.html` with status 200. Replace direct `http.FileServer` mount with this wrapper.
16. **CSP tighten:** drop `'wasm-unsafe-eval'` from CSP set in Phase 02 (no more WASM).
17. **Makefile:**
    - `web-install: cd web && npm ci`
    - `web-build: cd web && npm run build`
    - `web-dev: cd web && npm run dev`
    - `proto-gen: buf generate proto` (now emits both Go + TS)
    - `dev: web-dev & make server-dev` (or document as two terminals).
18. **CI:** add Node 20 setup, `npm ci` in `web/`, `npm run build`, then Go tests run as before. Confirm `web/dist/` artifact uploaded for deploy phase.
19. **Smoke:**
    - `make web-dev` → :5173.
    - `make dev` (server) → :8080.
    - Browser :5173 → sign in via Google (emulator) → WS connects → ping/pong logged.
    - `npm run build` → `web/dist/index.html` exists → server `:8080/` serves it → SPA route `/match/foo` falls back to index.html.

## Todo List
- [x] Delete `client/` Ebitengine module
- [x] Delete `web/wasm_exec.js`, `web/index.html`
- [x] Update `go.work` (remove client)
- [x] Scaffold SvelteKit + adapter-static
- [x] Install phaser, firebase, @bufbuild/protobuf
- [x] Configure svelte.config.js + vite.config.ts
- [x] Extend `buf.gen.yaml` with TS plugin
- [x] Verify generated TS protobuf compiles
- [x] Firebase init + emulator detection
- [x] auth-store with onAuthStateChanged
- [x] WS client with req-resp correlation + reconnect
- [x] Token refresh at 50min via AuthRefresh
- [x] PhaserGame component + EventBus
- [x] Title scene Phaser implementation
- [x] +layout.svelte auth gating
- [x] Sign-in component (3 providers)
- [x] Connection status indicator
- [x] Server `spa_fallback.go`
- [x] Tighten CSP (drop wasm-unsafe-eval)
- [x] Makefile web-* targets
- [x] CI Node + npm build step
- [x] .gitignore web/dist + node_modules
- [x] Bundle size measurement
- [ ] Manual smoke (sign-in → WS → ping) — skipped: no Firebase emulator + Mongo running locally

## Success Criteria
- [ ] `make web-build` produces `web/dist/index.html` + assets
- [ ] `make dev` (server only) serves `web/dist/index.html` at `/`
- [ ] `/match/foo` (non-existent route) returns `index.html` (SPA fallback)
- [ ] Anonymous sign-in → WS connects → app-level ping/pong round-trips
- [ ] Browser DevTools shows binary `Envelope` frames over `/ws`
- [ ] Token refresh at 50 min works (server logs confirm `AUTH_REFRESH`)
- [ ] Bundle <400 KB gzipped (visualizer report)
- [ ] CI green incl. npm build

## Risk Assessment
| Risk                                                   | Likelihood | Impact | Mitigation                                                       |
|--------------------------------------------------------|------------|--------|------------------------------------------------------------------|
| `protoc-gen-es` remote plugin requires Buf Schema Registry login on CI | Medium | Medium | Use local plugin via `npx`; docs in deployment-guide.md.        |
| Vite dev WS proxy drops binary frames                  | Low        | High   | Verify with smoke; Vite supports `ws:true` natively.             |
| Firebase JS SDK bundle blows past 400 KB               | Medium     | Medium | Use modular imports (`firebase/auth` only); tree-shaking on.     |
| SvelteKit prerender breaks if any route requires auth  | Medium     | Medium | Keep root prerender; auth-gate in `+layout.svelte` client-only.   |
| SPA fallback serves index.html for `.wasm` 404s        | Low        | Low    | Whitelist asset extensions (.js,.css,.png,...); only fallback for accept: text/html. |
| EventBus tight-coupling Phaser↔Svelte                  | Low        | Low    | Strict event names; document in code-standards.md.               |

## Security Considerations
- Firebase web config (`apiKey`, etc.) is public; safe to commit. Restrict by Firebase Auth Domain in console.
- ID tokens never logged client-side; stored in JS memory only (never localStorage).
- `Sec-WebSocket-Protocol` header is HTTPS-only in prod; tokens never traverse HTTP.
- CSP tightening drops `wasm-unsafe-eval`; verify no Phaser feature requires it.
- Service-worker cache (if added later) must exclude `/ws` and Firebase auth callbacks.
- `anonymous` sign-in is rate-limited by Firebase; no extra server-side limit needed at MVP.

## Next Steps
- Phase 07 — Game core pluggable + Wordle — depends on PhaserGame component + WS client + auth here.
