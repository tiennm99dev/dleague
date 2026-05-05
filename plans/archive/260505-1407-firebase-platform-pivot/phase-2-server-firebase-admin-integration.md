# Phase 2: Server — Firebase Admin SDK + WS auth gate

## Context Links
- Research: `plans/reports/researcher-260505-1407-firebase-as-backend-feasibility.md`
- Existing code: `server/internal/ws/conn.go:42` (Accept), `server/internal/ws/hub.go:46` (dispatch), `server/internal/store/store.go:32` (MySQL Store), `server/internal/config/config.go:36` (Load), `server/cmd/api/main.go:28` (boot)
- Proto: `proto/dleague/v1/envelope.proto:8` (MessageType enum)
- Locked: `MESSAGE_TYPE_AUTH_HELLO` first frame; ID token re-verified per WS upgrade; `STORE_BACKEND=firestore|mysql` env switch

## Overview
- **Priority:** P1 (server cannot accept authenticated traffic until done)
- **Status:** pending
- **Effort:** 4d
- Add `firebase.google.com/go/v4` + Admin SDK Firestore client. Add `auth` package (verify ID token + per-conn UID). Wire WS upgrade auth gate. Soft-deprecate (NOT delete) MySQL `store` package, gate by `STORE_BACKEND` env.

## Key Insights
- Admin SDK uses Application Default Credentials OR explicit credentials option. We pass `option.WithCredentialsJSON([]byte(envVar))` — single env, no file mount required for Coolify
- ID token TTL = 1h; client refreshes via `firebase/auth` JS SDK (`getIdToken(true)`); server MUST verify on **every** WS upgrade (cheap: cached cert in-process)
- Admin SDK calls bypass Firestore Security Rules — server is sole mutation authority
- Verifying ID token requires network call to Google for cert refresh (cached ~1h by SDK); first verify per process is slow, subsequent are local
- `firebase.google.com/go/v4` requires Go 1.20+ (we already have it)
- DO NOT trust client-provided UIDs in any payload field — always read `decodedToken.UID` after verify

## Requirements

### Functional
1. New env vars: `FIREBASE_PROJECT_ID`, `FIREBASE_CREDENTIALS_JSON` (required when `STORE_BACKEND=firestore`)
2. New env: `STORE_BACKEND` (default `firestore`; alternate `mysql`); `DATABASE_URL` becomes optional unless `STORE_BACKEND=mysql`
3. Add proto enum value `MESSAGE_TYPE_AUTH_HELLO = 3`, `MESSAGE_TYPE_AUTH_ACK = 4`, `MESSAGE_TYPE_AUTH_ERROR = 5`
4. New message types: `AuthHello { string id_token = 1; }`, `AuthAck { string uid = 1; int64 expires_unix_ms = 2; }`, `AuthError { string code = 1; string message = 2; }`
5. WS Conn rejects ALL frames except `AUTH_HELLO` until verified; closes with code 4401 if first frame is wrong type
6. After verify, conn stores `uid` + `tokenExpiresAt`; reads after expiry close conn with 4401
7. `/health` pings Firestore (replaces MySQL ping when `STORE_BACKEND=firestore`)
8. Firestore client wired to hub for dispatch handlers (phase 5–7 use it)

### Non-functional
- ID token verify <100ms p95 after first verify (cert cache warm)
- No goroutine leaks on conn close
- Each new file <200 LOC
- All new packages have unit tests (mock token verifier where Firebase SDK can't be hit)

## Architecture

### Auth flow
```
Client                              Server
------                              ------
WS upgrade → Accept (no auth check)
                                   conn.expectAuthHello = true
Send AUTH_HELLO{id_token}
                                   verify(id_token) via FirebaseAuth.VerifyIDToken
                                   conn.uid = decoded.UID
                                   conn.expectAuthHello = false
                                   reply AUTH_ACK{uid, expires_unix_ms}
Send PING / GAME_GUESS / etc.
                                   dispatch normally
... 1h passes ...
                                   token expired → close 4401
Client refresh → reconnect → AUTH_HELLO with fresh token
```

### Package layout
```
server/internal/
├── auth/
│   ├── verifier.go           # FirebaseVerifier{client, projectID}; VerifyIDToken(ctx, raw) (*Decoded, error)
│   ├── verifier_test.go      # uses interface mock
│   └── decoded.go            # Decoded { UID string; ExpiresAt time.Time; Email string; IsAnonymous bool }
├── firestore/
│   ├── client.go             # FsClient wraps *firestore.Client; New(ctx, projectID, credsJSON) (*FsClient, error)
│   ├── client_test.go        # boot/close test (skips if FIREBASE_CREDENTIALS_JSON unset)
│   └── health.go             # Ping(ctx) error — list 1 doc from /_meta/health
├── store/                    # KEEP (soft-deprecated)
│   └── ... (unchanged for now; gated in main.go)
├── ws/
│   ├── conn.go               # MODIFIED: handshake state machine
│   ├── auth_gate.go          # NEW: handleAuthHello + state struct
│   └── hub.go                # MODIFIED: dispatch ignores frames if !conn.authed
└── config/
    └── config.go             # MODIFIED: STORE_BACKEND, FIREBASE_*
```

### Files to create
- `server/internal/auth/verifier.go` (~80 LOC)
- `server/internal/auth/decoded.go` (~30 LOC)
- `server/internal/auth/verifier_test.go` (~80 LOC)
- `server/internal/firestore/client.go` (~70 LOC)
- `server/internal/firestore/health.go` (~30 LOC)
- `server/internal/firestore/client_test.go` (~50 LOC)
- `server/internal/ws/auth_gate.go` (~100 LOC)
- `server/internal/ws/auth_gate_test.go` (~120 LOC)

### Files to modify
- `server/internal/config/config.go` — add `StoreBackend`, `FirebaseProjectID`, `FirebaseCredentialsJSON` fields; relax `DatabaseURL` requirement
- `server/internal/ws/conn.go` — track `authed bool`, `uid string`, `tokenExp time.Time`; in `readLoop` reject non-AUTH_HELLO until authed; on token expiry close 4401
- `server/internal/ws/hub.go` — `dispatch` takes `*Conn` (not just envelope) so handlers see `conn.uid`; switch over `MESSAGE_TYPE_AUTH_HELLO` first
- `server/internal/http/health.go` — branch on store backend; ping Firestore when active
- `server/internal/http/router.go` — accept `firestore.FsClient` arg; pass to health handler
- `server/cmd/api/main.go` — boot Firestore client when `STORE_BACKEND=firestore`; boot MySQL Store only when `STORE_BACKEND=mysql`
- `proto/dleague/v1/envelope.proto` — extend enum, add 3 message types
- `shared/pb/dleague/v1/envelope.pb.go` — regenerated by `make proto-gen`
- `server/go.mod` — add `firebase.google.com/go/v4` and `cloud.google.com/go/firestore`

### Files to delete
- None this phase (MySQL stays; deletion deferred to phase-9)

## Implementation Steps
1. **Proto extension** — edit `proto/dleague/v1/envelope.proto`:
   - Add to `MessageType`: `MESSAGE_TYPE_AUTH_HELLO = 3; MESSAGE_TYPE_AUTH_ACK = 4; MESSAGE_TYPE_AUTH_ERROR = 5;`
   - Add 3 new message types as listed in Requirements §3
   - `make proto-gen`; commit regenerated `.pb.go`
2. **Auth package** — `server/internal/auth/verifier.go`:
   - Interface `Verifier interface { VerifyIDToken(ctx, raw string) (*Decoded, error) }`
   - Concrete `FirebaseVerifier` wraps `*firebaseauth.Client` from `firebase.google.com/go/v4/auth`
   - `New(ctx, projectID, credsJSON []byte) (*FirebaseVerifier, error)` uses `firebase.NewApp(ctx, &firebase.Config{ProjectID: projectID}, option.WithCredentialsJSON(credsJSON))`
3. **Firestore package** — `server/internal/firestore/client.go`:
   - `New(ctx, projectID, credsJSON []byte) (*FsClient, error)` uses `firestore.NewClient` with same options
   - `Client() *firestore.Client` exposes underlying handle for collection refs
   - `Close() error`, `Ping(ctx) error` (reads `_meta/health` doc; missing doc = OK as long as call succeeds)
4. **Config** — modify `server/internal/config/config.go`:
   - Add `StoreBackend string` (env `STORE_BACKEND`, default `firestore`)
   - Add `FirebaseProjectID string`, `FirebaseCredentialsJSON string`
   - Relax: `DatabaseURL` required only when `StoreBackend == "mysql"`
   - When `StoreBackend == "firestore"`: require `FirebaseProjectID` + `FirebaseCredentialsJSON`
5. **WS auth gate** — `server/internal/ws/auth_gate.go`:
   - Function `handleAuthHello(ctx, conn, env, verifier) error`
   - Unmarshal `AuthHello`, call `verifier.VerifyIDToken`, on success: set `conn.authed=true`, `conn.uid`, `conn.tokenExp`; reply `AUTH_ACK`
   - On failure: send `AUTH_ERROR{code, message}`; close conn with 4401
6. **WS conn** — modify `server/internal/ws/conn.go`:
   - Add fields to `Conn` struct: `authed bool`, `uid string`, `tokenExp time.Time`
   - In `handle`: if `!c.authed && env.Type != AUTH_HELLO` → close 4401
   - If `c.authed && now > c.tokenExp` → close 4401 (force reconnect with fresh token)
   - Pass `c` to `hub.dispatch` (signature change)
7. **WS hub** — modify `server/internal/ws/hub.go`:
   - `dispatch(c *Conn, env *Envelope, serverNow int64) (*Envelope, error)`
   - Add case `MESSAGE_TYPE_AUTH_HELLO` → calls `handleAuthHello`
   - Other handlers gain `conn.uid` for free (no client-trust)
8. **Main wiring** — modify `server/cmd/api/main.go`:
   - Branch on `cfg.StoreBackend`
   - Firestore branch: boot `auth.New(...)` + `firestore.New(...)`; pass to hub + health
   - MySQL branch: existing path unchanged
9. **Health** — modify `server/internal/http/health.go`:
   - Take new arg `fsHealth interface{ Ping(ctx) error }`
   - Branch ping target based on whichever is non-nil
10. **Router** — modify `server/internal/http/router.go` signature to accept verifier + fsClient + (legacy) store
11. **Tests:**
    - `auth/verifier_test.go` — wraps `Verifier` interface, mock returns canned `Decoded`
    - `ws/auth_gate_test.go` — mock verifier; assert: rejects non-AUTH_HELLO, accepts valid token, sends AUTH_ACK with correct UID, closes on bad token
    - `firestore/client_test.go` — skips with `t.Skip()` unless `FIREBASE_CREDENTIALS_JSON` env set (CI integration test)
12. **Compile check:** `cd server && go build ./...` — fix until clean
13. **Lint check:** `cd server && golangci-lint run`
14. **Test:** `make test`

## Todo List
- [ ] Extend `envelope.proto` with auth message types
- [ ] Run `make proto-gen` + commit `envelope.pb.go`
- [ ] Add deps `firebase.google.com/go/v4`, `cloud.google.com/go/firestore` to `server/go.mod`
- [ ] Create `server/internal/auth/decoded.go`
- [ ] Create `server/internal/auth/verifier.go` + tests
- [ ] Create `server/internal/firestore/client.go` + `health.go` + tests
- [ ] Modify `config/config.go`: add `StoreBackend`, Firebase fields, relax DB requirement
- [ ] Modify `ws/conn.go`: add auth state to Conn struct
- [ ] Create `ws/auth_gate.go` + tests
- [ ] Modify `ws/hub.go`: dispatch signature change, add AUTH_HELLO case
- [ ] Modify `http/health.go`: dual-backend ping
- [ ] Modify `http/router.go`: accept verifier + fsClient
- [ ] Modify `cmd/api/main.go`: backend-branched boot
- [ ] `go build ./...` + `golangci-lint run` + `make test`

## Success Criteria
- [ ] WS frame sent before AUTH_HELLO closes connection with code 4401
- [ ] Valid Firebase ID token (test via Firebase Auth REST emulator or live test user) → AUTH_ACK reply with correct UID
- [ ] Tampered/expired token → AUTH_ERROR + close
- [ ] Authenticated conn handles PING normally
- [ ] `/health` returns 200 when Firestore reachable, 503 when not
- [ ] `STORE_BACKEND=mysql` boot still works (regression: existing MySQL path unchanged)
- [ ] All new files <200 LOC
- [ ] Test coverage >80% on `auth/` and `ws/auth_gate.go`

## Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| `firebase.google.com/go/v4` first verify is slow (cert fetch) | High | Low | Warm in main() boot via `verifier.VerifyIDToken(ctx, "")` — error expected, but cert cached |
| ID token clock-skew between client/server causes false expirations | Med | Med | Tolerate ±60s skew (Admin SDK does this by default); document in code comment |
| `cloud.google.com/go/firestore` adds large dep tree (~30MB) | Low | Low | Acceptable; server binary still <80MB |
| Mock `Verifier` interface drifts from real Firebase shape | Med | Low | `Decoded` struct minimal (only UID, exp, email, anon flag); align with `auth.Token` fields |
| Conn struct field bloat | Low | Low | Group auth fields into nested `authState` struct |
| Race on `conn.authed` if dispatch reads while AUTH_HELLO writes | Med | Med | Auth state mutated only inside `handle()` — single goroutine per conn (readLoop); no race |
| MySQL store package leaks deps when `STORE_BACKEND=firestore` | Low | Low | Compile-time only; `go-sql-driver` registered globally but unused = no runtime cost |

## Security Considerations
- **Never log raw ID tokens** — they're bearer credentials; log only UID + masked token (first 8 chars)
- **Never trust client-supplied UID** in payload fields; always read from `conn.uid` set by verifier
- ID token has `aud` (audience) claim — Admin SDK validates against `FIREBASE_PROJECT_ID`; if mismatch, reject (cross-project token forgery)
- Service account JSON in env: Coolify masks env values in UI; verify masking on first deploy
- `auth_revoked` claim (Firebase token revocation) — `VerifyIDToken` doesn't check by default; pass `checkRevoked=true` for high-security ops (we skip at MVP, accept 1h max compromise window)
- Token replay across reconnects: same token may be reused within its 1h TTL; OK at MVP — collision impossible because UID-bound

## Next Steps
- **Unblocks:** phase-3 (data model can land via Admin SDK once client wired)
- **Unblocks:** phase-5/6/7 (game handlers can use `conn.uid` + `fsClient`)
- **Phase-4 dependency:** client must send `AUTH_HELLO` envelope as first frame after WS upgrade

## Unresolved Questions
1. Set `checkRevoked=true` on token verify? (cost: extra Firestore-like call per verify; benefit: instant ban) — decide before phase-9 production hardening
2. Should `AUTH_ACK` echo the user's display_name + photo_url to save a Firestore round-trip on connect? (Likely yes for UX; defer pattern decision to phase-5)
3. WebSocket close code 4401 (custom) vs 1008 (policy violation) — which to use? IETF says 4xxx is application-defined. Pick one in implementation.
