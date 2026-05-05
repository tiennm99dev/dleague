---
phase: 7
title: "React + Capacitor client scaffold"
status: pending
priority: P1
effort: "3d"
dependencies: [1, 6]
---

# Phase 7: React + Capacitor client scaffold

## Context Links

- Plan: [plan.md](plan.md)
- Capacitor Firebase plugin: https://capawesome.io/plugins/firebase/authentication/
- Prior pivot plan's similar phase: superseded `260505-1407-firebase-platform-pivot/phase-4-react-capacitor-client-bootstrap.md`

## Overview

Bootstrap a Vite + React + TypeScript app under `client/web/` (replacing Ebitengine WASM client direction). Wire Firebase Auth using `@capacitor-firebase/authentication` for cross-platform consistency. Implement the WS connection helper that performs the AUTH handshake from Phase 6. Web first; iOS/Android Capacitor wrappers stubbed but not built.

## Key Insights

- `@capacitor-firebase/authentication` v9+ supports Capacitor 7 (April 2025). Web uses Firebase JS SDK; native uses platform SDKs.
- Vite handles env vars via `import.meta.env.VITE_*`. Firebase client config (apiKey etc.) is non-secret — safe in bundle.
- Capacitor adds platforms only when needed: `npx cap add ios` / `npx cap add android` deferred.
- The existing `client/` directory contains an Ebitengine Go client — preserved for reference but not the build target.

## Requirements

- Functional: sign-in via email/password, Google, anonymous; obtain ID token; open WS to Go server; complete AUTH handshake; exchange ping-pong frames.
- **Beta-stage UI requirement:** SignIn page (and a sticky topbar after login) must display a **Beta banner**: _"🧪 Beta — your data may reset before public release. Early adopters earn rewards as thanks for joining now."_
- Non-functional: bundle ≤500 KB gzipped for landing screen; lazy-load auth flow.

## Architecture

```
client/web/
├── package.json
├── vite.config.ts
├── tsconfig.json
├── index.html
├── capacitor.config.ts          # added when iOS/Android target enabled
├── src/
│   ├── main.tsx                 # bootstrap
│   ├── auth/
│   │   ├── firebase.ts          # FirebaseAuthentication wrapper
│   │   └── useAuth.tsx          # React hook + context
│   ├── net/
│   │   ├── ws.ts                # WS open + AUTH handshake
│   │   └── proto.ts             # protobuf wire helpers (port from existing client/)
│   ├── pages/
│   │   ├── SignIn.tsx
│   │   └── Lobby.tsx
│   └── components/
└── public/
```

## Related Code Files

- Create:
  - `client/web/package.json`, `vite.config.ts`, `tsconfig.json`, `index.html`
  - `client/web/src/main.tsx`, `auth/{firebase,useAuth}.{ts,tsx}`, `net/{ws,proto}.ts`, `pages/{SignIn,Lobby}.tsx`
  - `client/web/src/components/BetaBanner.tsx` — sticky banner used on SignIn + after-login topbar
- Modify:
  - `go.work` — drop the `./client` member if Go-Ebitengine client is retired (or keep as reference)
  - `Makefile` — add `web-dev`, `web-build` targets
  - `.gitignore` — add `client/web/node_modules/`, `client/web/dist/`

## Implementation Steps

1. `mkdir client/web && cd client/web && npm create vite@latest . -- --template react-ts`
2. `npm i firebase @capacitor/core @capacitor-firebase/authentication`
3. `npm i -D @capacitor/cli` (defer `cap add ios/android` until mobile phase)
4. `src/auth/firebase.ts`: init Firebase app from `import.meta.env.VITE_FIREBASE_*`; expose `signInWithGoogle()`, `signInWithEmail()`, `signInAnonymously()`, `getIdToken()`.
5. `src/auth/useAuth.tsx`: React Context exposing `user`, `signIn`, `signOut`, `idToken`.
6. `src/net/proto.ts`: protobufjs runtime + decoded message helpers (mirror server's wire format).
7. `src/net/ws.ts`:
   - On open, send `AUTH_REQUEST{idToken}` immediately.
   - Wait for `AUTH_RESPONSE`; resolve connection promise on ok, reject on fail.
   - Auto-reconnect with token refresh via `getIdToken(true)`.
8. `components/BetaBanner.tsx`: dismissable-once-per-session banner reading "🧪 Beta — your data may reset before public release. Early adopters earn rewards as thanks for joining now." Render on SignIn page (always-visible) and as topbar on Lobby (collapsible).
9. `pages/SignIn.tsx` + `pages/Lobby.tsx`: minimal UI to prove the auth + WS path; SignIn shows BetaBanner above sign-in buttons; signup CTA copy includes "Join the beta — data may reset; early adopters get rewards."
9. Makefile: `web-dev: cd client/web && npm run dev`; `web-build: cd client/web && npm run build`.
10. Verify in browser: sign in with Google, see Lobby with ws-status="authenticated".

## Todo List

- [ ] Vite+React+TS scaffold
- [ ] Firebase JS SDK init
- [ ] @capacitor-firebase/authentication wrapper
- [ ] useAuth hook + context
- [ ] Proto wire helpers
- [ ] WS handshake helper
- [ ] BetaBanner component with copy approved by user
- [ ] SignIn page (3 providers) shows BetaBanner above provider buttons
- [ ] Lobby page proves WS auth + collapsible BetaBanner topbar
- [ ] Makefile targets
- [ ] go.work updated (or Ebitengine client retained as legacy)

## Success Criteria

- [ ] `make web-dev` boots dev server; sign in with Google; lobby shows `Authenticated as {email}` and `WS: connected`
- [ ] Anonymous sign-in path also works
- [ ] Token expires (60+ min) → ws helper refreshes silently and reconnects
- [ ] Production build (`npm run build`) under 500 KB gzipped initial bundle

## Risk Assessment

- **Capacitor 7 vs Vite 5 compat** — verify on first scaffold. Fallback: drop Capacitor for now, add only when mobile phase begins.
- **Protobufjs bundle size** — adds ~30 KB. Acceptable.
- **Token refresh race** — concurrent calls to `getIdToken` should dedupe; Firebase JS SDK does this internally.

## Security Considerations

- `apiKey` is public-by-design in Firebase; don't add it to .env-secret.
- Don't store ID tokens in `localStorage` if avoidable — Firebase JS SDK manages persistence; query via `getIdToken()` per use.
- CSP: allow `wss://<api-host>`, `https://*.googleapis.com`, `https://*.firebaseapp.com`.

## Next Steps

Phase 8 layers the pluggable game-engine on top. Phase 9/10 use this client for async/sync PvP.

## Unresolved Questions

- Retain `./client/` Ebitengine WASM client as fallback or delete? Decision: keep until Phase 12 cleanup; mark deprecated in README.
- Pull in Phaser 3 in this phase or later? Defer to Phase 8 spike.
