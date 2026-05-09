---
phase: 5
title: "Firebase Auth integration (server)"
status: completed
completed_on: 2026-05-09
priority: P1
effort: 1w
dependencies: [4]
---

# Phase 05 — Firebase Auth integration

## Context Links
- `plans/reports/researcher-260508-2300-firebase-admin-go-auth.md` (full integration guide)
- `server/internal/ws/conn.go` (Conn struct, UpgradeHandler in Phase 02)
- `server/internal/store/users.go` (UserRepo from Phase 04)
- `proto/dleague/v1/envelope.proto` (extend with AuthRefresh)

## Overview
Add server-side Firebase ID-token verification at WS upgrade via `Sec-WebSocket-Protocol` header. Auth-gate dispatch by adding `userID` + `isAnonymous` to `Conn`. Auto-create user doc in Mongo on first authenticated connection. Add `AuthRefresh` message for in-flight token rotation (1h expiry). Local dev via Firebase Emulator.

## Key Insights
- **Pattern A** (token in `Sec-WebSocket-Protocol`) chosen over post-upgrade auth message: token verified before any message dispatch happens — no DDoS surface (firebase report §2).
- ID tokens expire in 1 hour; long matches need `AuthRefresh` flow (firebase report §4). Client refreshes at 50 min; server re-verifies and updates `Conn.userID` + `tokenExpiresAt`.
- Anonymous auth supported — Firebase issues a UID with `firebase.sign_in_provider == "anonymous"` claim. Distinguish via `Conn.isAnonymous` (firebase report §6c).
- `VerifyIDToken` (no revocation) chosen for MVP; `VerifyIDTokenAndCheckRevoked` deferred to Phase 10 (firebase report §7).
- First-login auto-create: dispatcher calls `userRepo.UpsertByUID` with email/displayName from token claims (firebase report §5).
- Local dev: Firebase emulator (`firebase emulators:start --only auth`) on `127.0.0.1:9099`. Server detects via `FIREBASE_AUTH_EMULATOR_HOST` env (firebase report §9).
- Custom claims (admin/moderator) deferred to Phase 10.

## Requirements
**Functional:**
- New `server/internal/auth/firebase.go`: `Verifier` wrapping `*auth.Client`. Constructor takes Firebase project ID + creds path (or relies on emulator env).
- `Verifier.VerifyIDToken(ctx, idToken)` → `(*auth.Token, error)`.
- `UpgradeHandler` extracts `Sec-WebSocket-Protocol` parameter `fb.<idtoken>`, verifies, attaches `userID` + `isAnonymous` to `Conn`. On failure → 401.
- New proto `MESSAGE_TYPE_AUTH_REFRESH` + `AuthRefresh{id_token}` + `MESSAGE_TYPE_AUTH_REFRESH_ACK` + `AuthRefreshAck{expires_at_unix}`.
- Hub dispatcher routes `AuthRefresh` → `handleAuthRefresh(c, payload)` → re-verifies → updates `c.userID`, `c.tokenExpiresAt` → returns ack.
- `requiresAuth(env *Envelope) bool` helper; messages requiring auth (game/match types) rejected with `MESSAGE_TYPE_ERROR{code=401}` if `c.userID == ""`.
- On first authenticated message (or upgrade if no anonymous), `userRepo.UpsertByUID` writes user doc with `email`, `display_name`, `created_at`, `last_login`.
- `Conn.handle` passes `userID` + `isAnonymous` into dispatch context.

**Non-functional:**
- Boot fails fast if Firebase project ID / creds invalid (or emulator unreachable when `FIREBASE_AUTH_EMULATOR_HOST` set).
- `auth/firebase.go` <150 LOC.
- All auth paths covered by tests using emulator.
- No service account JSON committed.

## Architecture
```
client                                             server
─────                                             ──────
firebase JS SDK signs in
  ↳ user.getIdToken() → idToken
WS open: ws://.../ws
  Sec-WebSocket-Protocol: dleague.v1, fb.<idToken>
                          ────────────────────────►
                                                  UpgradeHandler:
                                                   ├─ extract fb.<...>
                                                   ├─ Verifier.VerifyIDToken
                                                   ├─ on err: 401
                                                   ├─ on ok: Conn{userID,isAnonymous,
                                                   │          tokenExpiresAt}
                                                   └─ userRepo.UpsertByUID

every dispatch:
  if requiresAuth(env) && c.userID=="":
    enqueue MESSAGE_TYPE_ERROR{401}; continue

at 50min mark, client:
  AuthRefresh{idToken}
                          ────────────────────────►
                                                  Hub.handleAuthRefresh:
                                                   ├─ Verifier.VerifyIDToken
                                                   ├─ update c.userID, expiresAt
                                                   └─ AuthRefreshAck
```

## Related Code Files
**Create:**
- `server/internal/auth/firebase.go` — Verifier wrapper
- `server/internal/auth/firebase_test.go` — emulator-gated tests
- `server/internal/ws/auth_refresh.go` — handler
- `server/internal/ws/auth_gate.go` — `requiresAuth` + dispatch gate
- `firebase.json` — emulator config (auth on :9099)
- `scripts/start-firebase-emulator.sh` — `firebase emulators:start --only auth`
- `scripts/stop-firebase-emulator.sh`

**Modify:**
- `server/cmd/server/main.go` — initialize `Verifier`; pass into `Hub`
- `server/internal/config/config.go` — `FirebaseProjectID string`, `FirebaseCredsPath string`, `FirebaseEmulatorHost string`
- `server/internal/ws/conn.go` — extract token from `Sec-WebSocket-Protocol`; populate `Conn.userID` + `isAnonymous` + `tokenExpiresAt`
- `server/internal/ws/hub.go` — store `verifier *Verifier`, `userRepo *UserRepo`; pass via constructor; `dispatch` signature includes `userID`
- `server/go.mod` — add `firebase.google.com/go/v4`
- `proto/dleague/v1/envelope.proto` — `MESSAGE_TYPE_AUTH_REFRESH`, `MESSAGE_TYPE_AUTH_REFRESH_ACK`, `AuthRefresh`, `AuthRefreshAck`
- `.github/workflows/ci.yml` — add Firebase emulator step (Node + firebase-tools install)
- `Makefile` — add `firebase-emulator` target
- `.env.example` — `FIREBASE_PROJECT_ID`, `FIREBASE_AUTH_EMULATOR_HOST=127.0.0.1:9099`, `GOOGLE_APPLICATION_CREDENTIALS` (commented for prod)
- `docs/code-standards.md` — verify no "session cookie" mentions remain
- `docs/system-architecture.md` — add auth flow section

**Delete:** none.

## Implementation Steps
1. **Proto:** add `MESSAGE_TYPE_AUTH_REFRESH = 5`, `MESSAGE_TYPE_AUTH_REFRESH_ACK = 6` enum values; new messages `AuthRefresh{string id_token = 1}` + `AuthRefreshAck{int64 expires_at_unix = 1}`. `make proto-gen`.
2. **Verifier:** `auth/firebase.go` — `New(ctx, projectID, credsPath)` returns `*Verifier`. Internally:
   - If `FIREBASE_AUTH_EMULATOR_HOST` env set: SDK auto-routes (no creds needed).
   - Else: `option.WithCredentialsFile(credsPath)` if non-empty; else default creds (Workload Identity for Fly.io).
   - `firebase.NewApp` → `app.Auth(ctx)` → store `*auth.Client`.
   - `Verifier.VerifyIDToken(ctx, idToken) (*auth.Token, error)` direct passthrough.
3. **UpgradeHandler:** add `extractToken(proto string) (string, error)` parser — split header by comma, find element prefixed `fb.`, return suffix. Reject if absent.
4. **Conn struct:** new fields `userID string`, `isAnonymous bool`, `tokenExpiresAt time.Time`. After verify, populate from `token.UID` and `token.Claims["firebase"].(map)["sign_in_provider"] == "anonymous"`.
5. **Auto-create user:** in `UpgradeHandler` after verify, call `userRepo.UpsertByUID(ctx, token.UID, Profile{Email: token.Claims["email"], DisplayName: token.Claims["name"], LastLogin: time.Now()})`. For anonymous users, omit email/displayName.
6. **Auth gate:** `auth_gate.go` defines `func requiresAuth(t MessageType) bool` — returns true for all message types except `PING`, `PONG`, `AUTH_REFRESH`, `AUTH_REFRESH_ACK`, `ERROR`. Hub.dispatch consults this; rejects unauth with `MESSAGE_TYPE_ERROR{code=401}`.
7. **AuthRefresh handler:** `auth_refresh.go` — re-verify token, update `c.userID`, `c.tokenExpiresAt`, return `AuthRefreshAck{expires_at_unix}`. On verify failure → `MESSAGE_TYPE_ERROR{401}` + close conn.
8. **Config:** add three new env vars + parse in `config.go`.
9. **Boot wiring:** `main.go` initializes `Verifier` after Mongo, passes both into `NewHub(verifier, userRepo)`.
10. **Firebase emulator:**
    - `firebase.json`: `{"emulators":{"auth":{"host":"127.0.0.1","port":9099}}}`
    - `scripts/start-firebase-emulator.sh`: `firebase emulators:start --only auth --project dleague-dev &`
    - Document `npm install -g firebase-tools` in deployment-guide.md skeleton.
11. **CI:** install `firebase-tools` (`npm i -g firebase-tools`); start emulator in background before `go test`. `FIREBASE_AUTH_EMULATOR_HOST=127.0.0.1:9099` exported.
12. **Tests:**
    - `firebase_test.go`: emulator-gated. Create test user via emulator REST API, generate ID token via custom-token exchange, verify via `Verifier.VerifyIDToken`.
    - `conn_test.go`: extend with upgrade-with-token + missing-token cases (mock verifier interface for unit-level).
    - `auth_refresh_test.go`: send `AuthRefresh` → expect `AuthRefreshAck`; send invalid token → expect `ERROR`.
13. **Manual:**
    - `firebase emulators:start --only auth`
    - Browser: sign in via emulator UI, copy ID token, `wscat -c ws://localhost:8080/ws -s "dleague.v1,fb.<token>"` → connection holds.
    - Without token: 401.

## Todo List
- [x] Proto: AUTH_REFRESH + ACK messages
- [x] Verifier wrapper
- [x] UpgradeHandler token extraction
- [x] Conn fields: userID, isAnonymous, tokenExpiresAt
- [x] UserRepo.UpsertByUID called on first auth
- [x] requiresAuth gate
- [x] AuthRefresh handler
- [x] Config: 3 new env vars
- [x] firebase.json + emulator scripts
- [x] CI emulator step
- [x] Tests: emulator-gated + unit
- [x] Manual smoke
- [x] Docs: system-architecture auth flow

## Success Criteria
- [ ] WS upgrade w/ valid token → 101; `Conn.userID` set
- [ ] WS upgrade w/o `fb.` protocol entry → 401
- [ ] WS upgrade w/ expired/forged token → 401
- [ ] First auth → user doc exists in Mongo with UID, email, last_login
- [ ] Anonymous auth → user doc with `is_anonymous: true`, no email
- [ ] `AuthRefresh` mid-conn → server updates `c.tokenExpiresAt`, returns ack
- [ ] Game/match message before auth → `ERROR{401}` (not auth → fine)
- [ ] CI emulator boots and tests pass

## Risk Assessment
| Risk                                                   | Likelihood | Impact | Mitigation                                                       |
|--------------------------------------------------------|------------|--------|------------------------------------------------------------------|
| `Sec-WebSocket-Protocol` blocked by proxy             | Low        | High   | Document; Fly.io passes through. Test on staging.                 |
| Token replay across hijacked WS                        | Low        | Medium | TLS-only in prod; defer revocation check to Phase 10.            |
| Emulator flake in CI                                   | Medium     | Medium | Wait-for-port script; fail fast if not ready in 30s.             |
| Auto-create race (two upgrades at once)                | Low        | Low    | `UpsertByUID` is idempotent (FindOneAndUpdate upsert).           |
| Token claim shape varies by provider                   | Medium     | Low    | Defensive map access; fall back to UID-only profile.             |
| Server mishandles AuthRefresh → user logs out         | Medium     | Medium | Tests cover happy + sad paths; client retries on ack timeout.    |

## Security Considerations
- TLS mandatory in prod (Fly.io default). Tokens in `Sec-WebSocket-Protocol` header — never logged.
- Service account JSON: `.gitignore`'d; loaded via `GOOGLE_APPLICATION_CREDENTIALS` (dev) or Workload Identity (prod).
- Anonymous user records: same `users` collection, flagged `is_anonymous: true`. Excluded from leaderboards (Phase 08).
- `requiresAuth` gate is conservative — explicit allowlist for unauth-able messages.
- Defer `VerifyIDTokenAndCheckRevoked` to Phase 10 (cost: ~100ms/call).
- Document accepted risk: Firebase outage → no fallback identity provider.

## Next Steps
- Phase 06 — Svelte+Phaser client scaffold — depends on this phase for WS auth contract.
