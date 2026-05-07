---
phase: 5
title: "Firebase Admin SDK + token verifier middleware"
status: in_progress
priority: P1
effort: "1d"
dependencies: [2]
---

# Phase 5: Firebase Admin SDK + token verifier

## Context Links

- Plan: [plan.md](plan.md)
- Firebase Admin Go: https://firebase.google.com/docs/admin/setup#go

## Overview

Server-side identity layer. Verify Firebase ID tokens on every WS connect and on protected HTTP endpoints. Upsert user records via `store.Store` (Phase 4) on first token-verify per UID. No server sessions table.

## Key Insights

- `firebase.google.com/go/v4` Admin SDK reads creds from JSON (env-injected here).
- ID token verification caches public keys (TTL ~6h) — first-token cost is one HTTPS round-trip, then memory-resident.
- Token claims used: `sub` (uid), `email`, `email_verified`, `firebase.sign_in_provider`, `name` (display name).
- ID tokens expire 1h. Client refreshes silently via Firebase JS SDK; server simply re-verifies on each connect/request.

## Requirements

- Functional: extract Bearer token from `Authorization` header (HTTP) or first WS frame (WS); verify; attach UID + email + provider to request context.
- Non-functional: verification < 5 ms after public-key cache warm; reject expired/invalid tokens with 401.

## Architecture

```
internal/auth/
├── firebase.go         # FirebaseAuth wrapper around firebase-admin
├── middleware.go       # http.Handler chi middleware: requires Bearer, attaches claims to ctx
└── ws_gate.go          # WS upgrade hook: read first AUTH frame, verify, attach claims
```

Flow:
```
client → HTTP Bearer JWT → middleware.Verify → ctx.WithValue(uidKey, claims) → handler
client → WS upgrade → conn open → first frame = AUTH{token} → ws_gate verifies → on fail close, on ok proceed
```

User upsert side-effect on first verify per UID — also stamps **beta-tester ledger** for future early-adopter rewards. Calls `store.UpsertUserOnFirstAuth` (interface defined in Phase 3, Couchbase impl owns persistence):
```
verify → claims → store.UpsertUserOnFirstAuth(claims) →
   subdoc INSERT isBetaTester=true, betaSignupAt=<now>      (only succeeds on first write)
   subdoc REPLACE lastSeen=<now>                             (always updates)
```
The store interface owns persistence — Phase 5 doesn't import `couchbase` or `redis` directly.

## Related Code Files

- Create:
  - `server/internal/auth/firebase.go`
  - `server/internal/auth/middleware.go`
  - `server/internal/auth/ws_gate.go`
  - `server/internal/auth/firebase_test.go`
- Modify:
  - `server/cmd/api/main.go` — init `auth.NewFirebase(ctx, cfg)` and pass to router
  - `server/internal/http/router.go` — accept `*auth.Firebase`, mount middleware on protected routes
  - `server/internal/ws/conn.go` (Phase 6 wires the AUTH frame; this phase just exposes the verifier)

## Implementation Steps

1. `cd server && go get firebase.google.com/go/v4@latest`
2. `firebase.go`: `NewFirebase(ctx, cfg) → *Firebase` parses JSON creds, calls `firebase.NewApp` then `app.Auth(ctx)`.
3. `Verify(ctx, token) (*Claims, error)` wraps `auth.VerifyIDToken`.
4. `middleware.go`: chi middleware extracts `Authorization: Bearer <jwt>`, calls `Verify`, on success calls `cb.Users.UpsertIfMissing(claims)` and stuffs claims into ctx.
5. `ws_gate.go`: exposes `Verify(token)` for the WS-upgrade path; Phase 6 wires it into the conn handshake.
6. Tests: unit-test middleware with a fake verifier (interface seam); skip live network in CI.

## Todo List

- [x] firebase-admin Go added (firebase.google.com/go/v4)
- [x] `auth.Verifier` interface + `auth.Firebase` concrete impl wrapping `app.Auth().VerifyIDToken`
- [x] HTTP Bearer middleware — extracts header, calls Verify, runs upsert side-effect, attaches claims via context key
- [x] WS gate verifier — `auth.Gate{}` exposes Verify(ctx, token); Phase 6 wires into AUTH frame protocol
- [x] User upsert on first verify — middleware + gate both call `Upserter.UpsertUserOnFirstAuth`; first write stamps beta fields, subsequent are no-ops on those fields
- [x] Tests with fake verifier — 5 cases (missing/malformed/invalid/valid/gate) all green; memstore validates upsert side effect end-to-end
- [ ] Wired into router for protected routes — deferred until Phase 6 lands (no protected HTTP routes yet; only `/health` + `/ws` exist; auth applies once async PvP REST + WS-AUTH frame appear)

## Success Criteria

- [ ] Valid token → 200 + user doc upserted
- [ ] Expired token → 401 with `WWW-Authenticate: Bearer error="invalid_token"`
- [ ] Tampered signature → 401
- [ ] Anonymous-provider token → user upserted with `provider: anonymous`, `email: ""`
- [ ] First verify per UID stamps `isBetaTester: true` + `betaSignupAt`; subsequent verifies don't overwrite
- [ ] `go test ./server/internal/auth/...` green

## Risk Assessment

- **Public-key cache cold start** — first verify after restart hits Google. Mitigation: warm with a synthetic verify on boot (verify a known dev token, ignore result).
- **Token clock skew** — Firebase tokens have 5-min leeway. Server NTP must be sane; otherwise spurious 401s.
- **Anonymous user spam** — anonymous sign-in lets anyone create users. Mitigation: rate-limit user upserts per IP; clean up anonymous users after 30d inactivity (cron).

## Security Considerations

- Only verify on TLS-terminated routes (Coolify handles TLS).
- Never log full JWT — only `uid` + first 8 chars of token for trace.
- `FIREBASE_PROJECT_ID` mismatched with token issuer → reject (Admin SDK does this automatically when initialized with creds).

## Next Steps

Phase 6 wires `ws_gate.Verify` into the WS upgrade flow. Phase 9 protects async PvP REST endpoints. Note: Phase 3 must define `store.AuthClaims` + `store.UpsertUserOnFirstAuth` (Couchbase impl) for this phase to consume — coordinate.

## Unresolved Questions

- Do we mint custom tokens for in-game admin actions (e.g. dev impersonation)? Defer.
- Refresh-token revocation: Firebase Auth supports token-revocation by setting `revoked: true` on user; server needs `--check-revoked` flag on Verify. Cost: extra Firestore read per verify. Skip until needed.
