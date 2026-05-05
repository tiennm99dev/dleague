---
phase: 7
title: "Svelte 5 + Phaser 4 + Capacitor client scaffold"
status: pending
priority: P1
effort: "3d"
dependencies: [1, 6]
---

# Phase 7: Svelte 5 + Phaser 4 + Capacitor client scaffold

## Context Links

- Plan: [plan.md](plan.md)
- Engine fitness research: [researcher-260505-1728-dle-platform-engine-fitness.md](../reports/researcher-260505-1728-dle-platform-engine-fitness.md)
- Official starter: https://github.com/phaserjs/template-svelte
- Capacitor Firebase plugin: https://capawesome.io/plugins/firebase/authentication/

## Overview

Bootstrap the client at `client/web/` from the official **`phaserjs/template-svelte`** (Vite + Svelte 5 + TypeScript + Phaser 4 + bridge + HMR). Wire Firebase Auth via `@capacitor-firebase/authentication`. Implement the WS connection helper performing the AUTH handshake from Phase 6. Web first; iOS/Android Capacitor wrappers stubbed but not built.

## Key Insights

- Official Phaser+Svelte template ships pre-wired with EventBus pattern for **Phaser ↔ Svelte communication** (Svelte calls Phaser scene methods; Phaser emits events Svelte subscribes to). No glue code needed.
- Svelte 5 runes (`$state`, `$derived`, `$effect`) replace stores for most use cases; Svelte stores still useful for cross-component shared state (auth, online presence).
- `@capacitor-firebase/authentication` v9+ supports Capacitor 7. Web uses Firebase JS SDK; native uses platform SDKs.
- Vite handles env vars via `import.meta.env.VITE_*`. Firebase client config (apiKey etc.) is non-secret — safe in bundle.
- Capacitor adds platforms only when needed: `npx cap add ios` / `npx cap add android` deferred until mobile phase.
- Bundle target ≤ 1 MB gzipped (Svelte ~10 KB + Phaser 4 ~345 KB + game logic + Firebase JS SDK ~80 KB + protobufjs ~30 KB ≈ ~600–800 KB).

## Requirements

- Functional: sign-in via email/password, Google, anonymous; obtain ID token; open WS to Go server; complete AUTH handshake; exchange ping-pong frames.
- **Beta-stage UI requirement:** SignIn page (and a sticky topbar after login) must display a **Beta banner**: _"🧪 Beta — your data may reset before public release. Early adopters earn rewards as thanks for joining now."_
- Non-functional: bundle ≤ 800 KB gzipped for landing screen; lazy-load auth flow + Phaser scene per game variant.

## Architecture

```
client/web/
├── package.json
├── vite.config.ts
├── svelte.config.js
├── tsconfig.json
├── index.html
├── capacitor.config.ts          # added when iOS/Android target enabled
├── src/
│   ├── main.ts                  # Svelte 5 app bootstrap
│   ├── App.svelte               # root layout
│   ├── PhaserGame.svelte        # canvas mount + EventBus from template
│   ├── EventBus.ts              # template-provided Svelte ↔ Phaser bridge
│   ├── auth/
│   │   ├── firebase.ts          # FirebaseAuthentication wrapper
│   │   └── auth.svelte.ts       # rune-based auth state ($state)
│   ├── net/
│   │   ├── ws.ts                # WS open + AUTH handshake
│   │   └── proto.ts             # protobuf wire helpers
│   ├── routes/                  # SvelteKit-style or simple file router
│   │   ├── SignIn.svelte
│   │   └── Lobby.svelte
│   ├── components/
│   │   └── BetaBanner.svelte    # sticky banner
│   └── games/                   # populated in Phase 8
└── public/
```

Phaser ↔ Svelte data flow:
```
Svelte App (auth, lobby, HUD)
   │
   ├─ EventBus.emit('start-game', puzzle)  →  Phaser scene reacts
   ├─ EventBus.on('attempt-complete', cb)  ←  Phaser scene emits result
   └─ <PhaserGame /> mounts the Phaser canvas
```

## Related Code Files

- Create:
  - `client/web/package.json`, `vite.config.ts`, `svelte.config.js`, `tsconfig.json`, `index.html`
  - `client/web/src/main.ts`, `App.svelte`, `PhaserGame.svelte`, `EventBus.ts`
  - `client/web/src/auth/{firebase.ts,auth.svelte.ts}`
  - `client/web/src/net/{ws.ts,proto.ts}`
  - `client/web/src/routes/{SignIn,Lobby}.svelte`
  - `client/web/src/components/BetaBanner.svelte`
- Modify:
  - `go.work` — drop `./client` member if Go-Ebitengine WASM client retired (decision in Phase 12)
  - `Makefile` — add `web-dev`, `web-build` targets
  - `.gitignore` — add `client/web/node_modules/`, `client/web/dist/`

## Implementation Steps

1. `npx degit phaserjs/template-svelte client/web && cd client/web && npm install` — pulls the official Phaser 4 + Svelte 5 + Vite + TS scaffold.
2. `npm i firebase @capacitor/core @capacitor-firebase/authentication`
3. `npm i -D @capacitor/cli` (defer `cap add ios/android` until mobile phase)
4. `src/auth/firebase.ts`: init Firebase app from `import.meta.env.VITE_FIREBASE_*`; expose `signInWithGoogle()`, `signInWithEmail()`, `signInAnonymously()`, `getIdToken()`.
5. `src/auth/auth.svelte.ts`: rune-based singleton — `$state`-backed `user`, `idToken`; reactive functions `signIn`, `signOut`.
6. `src/net/proto.ts`: protobufjs runtime + decoded message helpers (mirror server's wire format).
7. `src/net/ws.ts`:
   - On open, send `AUTH_REQUEST{idToken}` immediately.
   - Wait for `AUTH_RESPONSE`; resolve connection promise on ok, reject on fail.
   - Auto-reconnect with token refresh via `getIdToken(true)`.
8. `components/BetaBanner.svelte`: dismissable-once-per-session banner. Render on SignIn page (always-visible) and as topbar on Lobby (collapsible).
9. `routes/SignIn.svelte` + `routes/Lobby.svelte`: minimal UI to prove the auth + WS path. SignIn shows BetaBanner above sign-in buttons; signup CTA copy includes "Join the beta — data may reset; early adopters get rewards."
10. Verify in browser: sign in with Google → see Lobby with `Authenticated as {email}` and `WS: connected` → "Play today's puzzle" button kicks off the Phaser scene (Phase 8 populates the actual game).
11. `Makefile`: `web-dev: cd client/web && npm run dev`; `web-build: cd client/web && npm run build`.

## Todo List

- [ ] `phaserjs/template-svelte` scaffolded into `client/web/`
- [ ] Firebase JS SDK init
- [ ] @capacitor-firebase/authentication wrapper
- [ ] Rune-based auth state (`auth.svelte.ts`)
- [ ] EventBus pattern verified Svelte ↔ Phaser
- [ ] Proto wire helpers
- [ ] WS handshake helper
- [ ] BetaBanner component with copy approved
- [ ] SignIn page (3 providers) shows BetaBanner above provider buttons
- [ ] Lobby page proves WS auth + collapsible BetaBanner topbar
- [ ] Makefile targets
- [ ] go.work updated (or Ebitengine client retained as legacy until Phase 12 decision)

## Success Criteria

- [ ] `make web-dev` boots Vite dev server; sign in with Google; lobby shows `Authenticated as {email}` and `WS: connected`
- [ ] Anonymous sign-in path also works
- [ ] Token expires (60+ min) → ws helper refreshes silently and reconnects
- [ ] Production build (`npm run build`) under 800 KB gzipped initial bundle
- [ ] Phaser canvas mounts without warnings; EventBus emit/on round-trip works

## Risk Assessment

- **Capacitor 7 + Phaser 4 + Svelte 5 compat** — verify on first scaffold. Official template targets Phaser 4 + Svelte 5; Capacitor wrap is framework-agnostic. Low risk.
- **Bundle bloat** — Phaser 4 (345 KB) is the lion's share. Tree-shake unused Phaser features (e.g. drop tilemap if no game uses it).
- **Token refresh race** — concurrent calls to `getIdToken` should dedupe; Firebase JS SDK does this internally.
- **Svelte 5 runes vs stores** — runes are newer; some npm libs assume old store API. Stick with stores for cross-cutting state where libs need them; runes for component-local.

## Security Considerations

- `apiKey` is public-by-design in Firebase; don't add it to .env-secret.
- Don't store ID tokens in `localStorage` if avoidable — Firebase JS SDK manages persistence; query via `getIdToken()` per use.
- CSP: allow `wss://<api-host>`, `https://*.googleapis.com`, `https://*.firebaseapp.com`.

## Next Steps

Phase 8 layers per-variant Phaser scenes + Svelte HUDs on top of this scaffold. Phase 9/10 use the WS connection for async/sync PvP.

## Unresolved Questions

- Retain `./client/` Ebitengine WASM client as fallback or delete? Decision in Phase 12; latest research strongly recommends delete.
- Phaser 4 build customization to drop unused features (tilemap, particles for v1) — reduce bundle further. Defer to optimization pass post-Phase 8.
