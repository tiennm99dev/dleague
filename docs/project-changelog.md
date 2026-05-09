# Project Changelog

All notable changes to Dleague. Most recent first. Commits reference the main branch.

---

## Phase 10 — Deploy + polish (pending commit)

- **Deploy artifacts:** `Dockerfile` multi-stage (golang:1.23-alpine → node:20-alpine → distroless/static), `fly.toml` (app=dleague, region=iad, min_machines=1), `scripts/set-fly-secrets.sh`, `scripts/seed-wordlists.sh`, `scripts/promote-admin.sh`.
- **Boot-time secret decode:** `FIREBASE_SERVICE_ACCOUNT_B64` decoded to `/tmp/dleague-sa.json` at startup; `GOOGLE_APPLICATION_CREDENTIALS` set automatically. Service-account JSON never embedded in image.
- **Admin CLI:** `server/cmd/admin/main.go` — `promote-admin` sets Firebase custom claim `admin:true`; `revoke-token` revokes refresh tokens.
- **Auth package:** Added `Auth.Admin`, `SetAdminClaim`, `RevokeRefreshTokens`, `VerifyIDTokenAndCheckRevoked`. `newApp` extracted to reduce duplication.
- **`Conn.isAdmin`:** Populated from `token.Claims["admin"].(bool)` on WS upgrade.
- **CI SHA pinning:** All `uses:` in `.github/workflows/ci.yml` replaced with immutable commit SHAs.
- **Dependabot:** `.github/dependabot.yml` — weekly digest updates for GitHub Actions, `gomod` (server + shared), `npm` (web).
- **Queue self-pair guard (Phase 09 M2):** `PopPair` returns `ok=false` if both front entries share the same `userID`.
- **Queue 60 s TTL (Phase 09 M6):** `queueEntry.enqueuedAt` field; `EvictExpired` method; background goroutine in `main.go` fires every 5 s, sends `ERROR{408, "queue_timeout"}`.
- **`CompleteSync` idempotent (Phase 09 M5):** Filter adds `state:"active"` guard; re-resolution is a silent no-op.
- **`cryptoSeed` fail-loud (Phase 09 L1):** `log.Printf` + `os.Exit(1)` on `rand.Read` error instead of returning `42`.
- **Session eviction (Phase 07 M3):** Solo `wordleSession` deleted from `sync.Map` on terminal game state and on conn disconnect.
- **EventBus listener cleanup (Phase 07 M5):** `WordleScene.shutdown()` calls `eventBus.off('wordle:flip-row', handler)`.
- **`tokenToProfile` non-destructive upsert (Phase 05 M3):** Only non-empty claims written; existing `display_name`/`avatar_url` not overwritten by anonymous login.
- **Email field (Phase 05 M4):** `UserProfile.Email` and `User.Email` added; persisted from `claims["email"]` when present.
- **Docs sweep:** All `docs/*.md` files filled — no `TODO` markers remaining. Deployment guide, design guidelines, roadmap, PDR, changelog, code standards cleanup.
- **Makefile:** `deploy`, `deploy-staging`, `seed-wordlists-prod` targets added.
- **`.gitignore`:** `.fly/` appended.

---

## Phase 09 — Sync PvP (`70c8904`)

- In-memory matchmaking `Queue` (FIFO, per-game-ID).
- `RoomsRegistry` — concurrent-safe map of live match rooms.
- `MatchRoom` — per-room `sync.Mutex`; `HandleMove`, `HandleForfeit`, `HandleTimeout`.
- `GraceTimers` — 30 s disconnect grace with `time.AfterFunc`.
- New envelope types: `QUEUE_JOIN/LEAVE/MATCHED`, `MATCH_MOVE`, `MATCH_OPPONENT_PROGRESS`, `MATCH_RESOLVED`, `MATCH_REJOIN/ACK`.
- Letters-never-leak invariant: opponent sees color arrays only, no guess strings.
- `MatchRepo.CreateSync` / `CreateSyncWithID` — pre-generated ObjectID eliminates orphan-match window.
- `MatchRepo.CompleteSync` — Mongo transaction for atomic match-end.
- Per-conn rate limiter: 10-token burst, 10/s refill.
- 5 min match timeout via 1 s ticker in `main.go`.
- Tests: `TestMatchRoom_LettersNeverLeakToOpponent`, `TestQueue_*`, `TestGraceTimers_*`.

---

## Phase 08 — Async PvP (`9f66faa`)

- Challenge link flow: `CREATE_MATCH` → share token → `JOIN_MATCH` → both players attempt same seed.
- `MatchRepo` and `AttemptRepo` backed by Mongo.
- `LeaderboardRepo` with pre-computed ranking snapshots; `scheduler` refreshes every 5 min, sweeps expired matches every 15 min.
- `LEADERBOARD_QUERY` handler returns top-N with rank + attempts + solve time.
- `MATCH_RESULT` handler compares both players' attempts and returns winner.
- Mongo transactions for `JoinAsChallengee` (optimistic lock on `challengee_uid`).
- Anonymous users excluded from leaderboard updates.

---

## Phase 07 — Game core pluggable + server-authoritative Wordle (`a4b3762`)

- `server/internal/game/wordle/` — `Wordle` struct with `New`, `Validate`, `Apply`, `ToProto`, `Score`.
- Two-pass color scoring: counts letter frequencies, correct-position first, then present.
- `WordleState.solution` hidden in pre-terminal responses; revealed on won/lost.
- `daily_puzzles` collection + `EnsureToday` with SHA-256 seeding.
- `wordlists` collection + embedded fallback (`data/answers.txt`, `data/dictionary.txt`).
- `cmd/seed-wordlists` CLI for uploading word lists to Mongo.
- `shared/game.Game` interface + `Registry` factory.
- EventBus (`event-bus.ts`) — typed pub/sub for Phaser → Svelte communication.
- `WordleScene` — Phaser Y-axis tile-flip animation, staggered per column.
- `board.svelte`, `keyboard.svelte` — DOM-based Wordle UI.
- Tests: `TestWordle_*`, `TestToProto_SolutionHiddenPreTerminal`, 60+ assertions.

---

## Phase 06 — SvelteKit + Phaser client scaffold (`5f837ea`)

- Drop Ebitengine WASM; new stack: SvelteKit 2 (adapter-static) + Phaser 3.88.
- Firebase JS SDK v11 — `initializeApp`, `connectAuthEmulator`, `signInWithPopup`.
- `auth-store.ts` — `writable<User|null>`, `onAuthStateChanged`, `idToken()`.
- `ws.ts` — binary protobuf WS client; exponential-backoff reconnect; `requestId` correlation.
- `sign-in.svelte` — email/password + Google popup + anonymous.
- SvelteKit routes: `/` (home), `/play` (daily), `/leaderboard`, `/quick-match`, `/sync`, `/m/[token]` (challenge join).
- Vite proxy: `/ws` and `/health` forwarded to `:8080` in dev.
- CSP `wasm-unsafe-eval` removed (no WASM).
- Bundle: ~411 KB gzip total (over 400 KB target; Phaser custom build deferred to v2).

---

## Phase 05 — Firebase Auth integration (`dd89f9c`)

- `server/internal/auth/firebase.go` — `Verifier.VerifyIDToken`; ADC + emulator support.
- ID token piped via `Sec-WebSocket-Protocol: dleague.v1, fb.<token>`.
- `Conn.userID`, `Conn.isAnonymous`, `Conn.tokenExpiresAt` populated at upgrade.
- `tokenToProfile` maps token claims → `UserProfile` for Mongo upsert.
- `AuthRefresh` handler refreshes token mid-connection without re-upgrade.
- `requiresAuth` dispatch gate on all game/match message types.
- CI: Firebase emulator started before tests; `FIREBASE_AUTH_EMULATOR_HOST` set.

---

## Phase 04 — MongoDB store rewrite (`54a037d`)

- `store.Connect` — `*mongo.Client` wrapper with timeout + `Ping`.
- Per-collection repos: `UserRepo`, `GameRepo`, `MatchRepo`, `AttemptRepo`, `DailyPuzzleRepo`, `WordlistRepo`, `LeaderboardRepo`.
- `store.EnsureIndexes` — 8 explicit indexes created at boot (idempotent).
- `schema_version` field on all documents; lazy-migration strategy.
- Integration tests with `MONGO_TEST_URI` env var.

---

## Phase 03 — WS lib migration nhooyr → coder/websocket (`f5b9c9d`)

- Replaced archived `nhooyr.io/websocket` with maintained `github.com/coder/websocket`.
- API is API-compatible; minimal code changes.
- `go.mod` updated; old dependency removed.

---

## Phase 02 — Server hardening (`ea677a3`)

- `Hub` with `sync.RWMutex` replacing `sync.Mutex`; bounded `send` channels.
- `WriteTimeout`, `ReadHeaderTimeout`, `IdleTimeout` on `http.Server`.
- `request_id` length cap (128 bytes) to prevent log injection.
- Origin allowlist (`DLEAGUE_WS_ORIGINS`); boot assertion in production.
- Security headers middleware (CSP, HSTS, X-Frame-Options, etc.).
- `RateLimiter` per-conn token bucket (golang.org/x/time/rate).
- `graceful shutdown` — `hub.CloseAll` before `srv.Shutdown`.

---

## Phase 01 — Archive + docs bootstrap (`1819359`)

- Archived superseded plans to `plans/archive/`.
- Created `docs/` skeleton: system-architecture, codebase-summary, code-standards,
  deployment-guide, design-guidelines, roadmap, PDR, changelog.
- Platform pivot decision: Svelte+Phaser+Firebase+MongoDB (from Ebitengine+WASM+Postgres).
- Decision record: `plans/reports/decision-record-260505-1407-platform-pivot.md`.

---

## Phase 00 — Foundation (`9937c7d`)

- Go workspace (`go.work`) with `server/` and `shared/` modules.
- Protobuf wire: `proto/dleague/v1/envelope.proto`; `buf generate` for Go + TS.
- `/health` endpoint returning `{"status":"ok"}`.
- `/ws` WebSocket endpoint with ping-pong frame handling.
- `debug` build tag: protojson logging on both server (stdout) and client (console).
- Initial `Makefile` with `dev`, `build`, `test`, `proto-gen`, `tools` targets.
