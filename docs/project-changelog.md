# Project Changelog

All notable changes to Dleague. Most recent first. Commits reference the main branch.

---

## Post-MVP Hardening — Phase 04 Pluggability decision (2026-05-09)

- **Proto enum rename:** `MESSAGE_TYPE_GAME_MOVE` → `MESSAGE_TYPE_WORDLE_MOVE`, `MESSAGE_TYPE_GAME_STATE` → `MESSAGE_TYPE_WORDLE_STATE`. Numeric values (6/7) preserved; comment added to proto documenting wordle-only semantics. Preempts future `buf breaking` violations.
- **Dead code cleanup:** `server/internal/store/games.go` fully deleted (unused GameRepo); `_ = store.NewGameRepo(db)` line removed from `server/cmd/api/main.go:95`; unused `KeyEnter`/`KeyBackspace` consts dropped from `shared/game/game.go:38-40`.
- **Scaffold documentation:** Top-of-file comments added to `shared/game/game.go` and `registry.go` explicitly reserving the interface for v2 multi-game support. Current release ships Wordle-only.
- **Doc claims updated:** README, PDR, codebase-summary, and development-roadmap reworded to demote "pluggable game types" from present-tense feature to v2/exploratory roadmap item. `system-architecture.md` updated to remove Game interface diagram; retains wordle-specific dispatch table (refreshed in Phase 07).
- **Callsite updates:** Proto regeneration (`make proto-gen`) updated Go references across hub, game_handler, match_room, match_room_test, sync_match_handler; TS references updated in play, sync-game-scene, ws.ts comment.
- **Test results:** `go build/vet` clean; race-detector 6 packages; svelte-check 0/0; 9 web tests pass.

---

## Post-MVP Hardening — Phase 05 Persistence & data integrity (2026-05-09)

- **State-filter audit:** All state-mutating `UpdateOne`/`FindOneAndUpdate` operations across `store/` package now filter on source state. `JoinAsChallengee` adds `state:"pending"` guard; `Complete` and `CompleteSync` already filtered. Comments mark each: `// MUST: filter on source state to prevent double-resolve`.
- **Leaderboard threshold guard:** Hard cap at 5000 matches/day; `Refresh` queries `countDocuments()` first. If exceeded, logs WARN and returns sentinel `ErrLeaderboardTooLarge` (scheduler continues). Explicit boundary prevents silent O(N) memory growth; scale-out path documented (aggregation pipeline via `$lookup`).
- **`Attempt.Hints` field dropped:** Field existed but was never written by any code path. Rationale: replay-from-guesses computable on-demand if anti-cheat needs it (re-run `wordle.Score` over `attempt.guesses[]`). No data migration needed (field was always empty).
- **Atlas tier documented:** system-architecture.md adds "Atlas tier requirements" subsection: M0 free tier (500 cluster-wide conns, 100 per user) sufficient for dev only; production requires M10+ (1500 max conns/cluster). Connection pool (100 max, 10 min) sized for M10; scale to 200/20 on M20+ tier.
- **`parseDBName` fail-fast:** Malformed `MONGO_URI` now fails at boot before listening. Returns `(string, error)` from parse; caller in `Connect` propagates upstream to `main.go`. Caveat: `mongo.Connect.ApplyURI` already validates upstream; this is defensive redundancy (mostly cosmetic but harmless).
- **Index naming audit:** All 9 indexes named consistently and self-documenting: `attempts_match_player_unique`, `matches_share_token_unique`, `users_display_name_unique`, etc. Names visible in `db.collection.getIndexes()` output for operator clarity.
- **Test results:** `go build/vet ./...` pass; `go test -race` clean across store + scheduler + ws packages. Server build + web svelte-check both green.

---

## Post-MVP Hardening — Phase 03 UX correctness (2026-05-09)

- **Mid-match navigation gating:** Rejoin logic gated to landing routes (`/`, `/play`, `/leaderboard`); `inMatch` flag prevents auto-navigation when user is rendering live sync match on `/sync`.
- **Board accessibility:** Replaced `role="grid"` with `role="region"` + `aria-live="polite"`; dropped per-cell empty-state noise; added per-row status summaries for screen readers.
- **On-screen keyboard focus:** Added `tabindex="-1"` to all keys; `onpointerdown` preventDefault prevents focus steal and scroll jump.
- **Anonymous-user warning:** New `anonymous-warning.svelte` component mounted in sign-in form (inline) and sticky banner on `/play` if `authUser.isAnonymous`. Explains "scores not saved" to daily leaderboard.
- **Friendly sign-in errors:** Firebase error codes (`auth/wrong-password`, `auth/user-not-found`, `auth/invalid-email`, `auth/too-many-requests`) mapped to user-friendly messages; anti-enumeration: `wrong-password` and `user-not-found` both show "Incorrect email or password".
- **Reconnect affordance:** `connection-status.svelte` Reconnect button now visible when `state == disconnected`; disabled while connecting to prevent race. Button calls `connect(await idToken())`.
- **Results-screen edge cases:** Added `reason` prop supporting `'win' | 'loss' | 'tie' | 'opponent-left' | 'self-disconnect'`. Copy variants per reason; opponent-left suppresses "Challenge again" CTA.
- **Challenge-create busy guard:** `play/+page.svelte` wraps `CHALLENGE_CREATE` in `creating` state; button disabled during request to prevent double-clicks.
- **Test results:** Web `svelte-check` 0 errors/warnings across 400 files; 9 web tests pass; server 6 packages ok; race-clean test suite.

---

## Post-MVP Hardening — Phase 02 Security & abuse hardening (2026-05-09)

- **Log redaction:** UID redaction via HMAC-SHA256 per-process salt (`log_redact.go`); share token truncated to 8 chars in logs.
- **Per-UID rate limiting:** New `UIDLimiter` struct with TTL eviction (1h idle); wired into dispatch after auth gate (defence in depth over per-conn limiter).
- **Attempt bounds:** `AttemptSubmit` guesses array capped at 6 entries; rejects with 422 if exceeded.
- **Web auth UX:** `idToken(force = false)` parameter; 401 WS close triggers one force-refresh per minute; `AuthErrorToast` non-blocking alert instead of silent empty-token connect.
- **`displayName` privacy:** Sync-match opponent fallback → `"Player ${last4}"` strips UID from broadcast.
- **OriginPatterns doc:** boot-time warn if production config contains wildcards.
- **Test results:** `go test -race` 10/10 packages green (97 test runs); `svelte-check` 0 errors.

---

## Post-MVP Hardening — Phase 01 Critical Correctness (2026-05-09)

- **Queue stale connection fix:** Disconnect defer now calls `Queue.Remove(conn)` to prevent ghost matches.
- **Auth field race closure:** All `Conn.userID/isAnonymous/isAdmin/tokenExpiresAt` writes wrapped in `c.mu.Lock()`. New read accessors `UserID()`, `IsAnonymous()`, `IsAdmin()` ensure safe cross-goroutine access.
- **Server crash on crypto/rand error:** `cryptoSeed()` now returns `(int64, error)`; caller in `startSyncMatch` returns 500 instead of `os.Exit(1)`.
- **Duplicate attempt prevention:** Compound index `(match_id, player_uid)` now `Unique: true`. `AttemptRepo.Insert` handles 11000 code → `ErrAttemptExists`.
- **Idempotent match completion:** `MatchRepo.Complete` filters on `state:"pending"` guard; `IncrementStats` gates on `ModifiedCount == 1`.
- **Web lifecycle hoisted:** `connect()`/`disconnect()` calls moved to `+layout.svelte` and triggered by `authUser` subscription; per-route calls removed.
- **WS pending-promise rejection:** `onclose` now calls `rejectAllPending()` before any reconnect schedule, ensuring stale promises fail fast.
- **Rejoin payload threading:** New `match-rejoin-store.ts` carries `ownState`/`opponentHints` from `MATCH_REJOIN_ACK` into `/sync` route.
- **Sync Enter key fix:** On-screen keyboard now emits canonical `'Enter'`/`'Backspace'` casing; `sync-game-scene.svelte` comparisons normalized.
- **GAME_STATE deduplication:** Removed server push path; kept request-response only to eliminate double-dispatch.
- **Test results:** `go test -race` 83 tests pass; `svelte-check` 0 errors.

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
