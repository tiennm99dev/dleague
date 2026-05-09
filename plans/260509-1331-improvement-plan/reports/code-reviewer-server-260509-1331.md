# Server Review — Go Backend (260509-1331)

Scope: `server/cmd/{api,admin,seed-wordlists}`, `server/internal/{ws,auth,store,config,http,scheduler,game/wordle}`, `shared/game`. Deployment files explicitly skipped.

Lines reviewed: ~7,200 (incl. tests). Focus: correctness, security, perf, maintainability.

---

## High-impact issues

### H1. Disconnected player never removed from matchmaking queue → stale `*Conn` pointers + ghost matches
**Severity:** High (correctness, memory)
**Files:** `server/internal/ws/conn.go:160-172`, `server/internal/ws/sync_match_handler.go:66`, `server/internal/ws/queue.go:32-37`
**Why it matters:** `QUEUE_JOIN` calls `deps.Queue.Push(gameID, c)`. Only `handleQueueLeave` and `EvictExpired` call `Queue.Remove`. The disconnect defer in `conn.go:161` never touches the queue. So if a player joins the queue and then closes the tab without sending `QUEUE_LEAVE`, their stale `*Conn` sits in the FIFO until the 60s TTL fires. Worst case: `PopPair` returns one live conn + one dead conn → `startSyncMatch` enqueues `QueueMatched` to a closed `send` channel. The non-blocking `default` branch in `enqueue` (`conn.go:308`) calls `cancelRead()` on a defunct conn (no-op), but the live partner is now paired with a corpse and the room exists in registry until 5-min timeout.
**Fix:** in the disconnect defer in `conn.go:161-172`, call `hub.GameDeps.Queue.Remove(conn)` (guarded by nil-check). Cheap O(n) walk that already exists.

### H2. `Conn.userID` / `isAdmin` / `isAnonymous` mutation is racy after `AuthRefresh`
**Severity:** High (security/correctness)
**Files:** `server/internal/ws/auth_refresh.go:48-51`, `server/internal/ws/conn.go:34-37`, `server/internal/ws/hub.go:103`, `server/internal/ws/match_room.go:51`
**Why it matters:** `Conn.userID`, `isAnonymous`, `isAdmin`, `tokenExpiresAt` are read without a lock from the disconnect defer (`conn.go:170` reads `conn.userID`), `dispatch` (`hub.go:103`), `match_room.playerIndex` (`match_room.go:51`), and `match_room.HandleForfeit` (`match_room.go:140`). Writes happen on the readLoop goroutine (`auth_refresh.go:49`). Single-goroutine readLoop reads/writes are fine, but cross-goroutine reads (disconnect defer is the read goroutine, but `HandleForfeit` runs from `time.AfterFunc` in `disconnect.go:44`, and `HandleMove` reads `c.userID` while the WS connection serializes incoming frames per-conn — so cross-conn writes via `HandleMove` use the OTHER conn's userID, which is mutated only by the OTHER conn's readLoop). Net: there is a real race when conn A's `HandleMove` reads conn B's `userID` while conn B is processing `AuthRefresh`. `go test -race` would flag this. The auth-refresh check in `auth_refresh.go:42-46` rejects UID switches — so the data-loss is bounded — but reading torn strings is undefined.
**Fix:** put `userID`, `isAnonymous`, `isAdmin`, `tokenExpiresAt` behind `c.mu` (already exists). Add `Conn.UserID()` and use it from cross-goroutine readers. Or use `atomic.Pointer[string]`.

### H3. `RoomsRegistry.Add` can clobber an active room (no `LoadOrStore`)
**Severity:** High (data loss)
**Files:** `server/internal/ws/match_rooms_registry.go:17-21`, `server/internal/ws/sync_match_handler.go:200-218`
**Why it matters:** `Add` unconditionally overwrites. If two `QUEUE_JOIN` flows somehow produce the same `matchID` (collision astronomically unlikely with `bson.NewObjectID`, but possible if a bug ever feeds a fixed/test ID, or if a `MATCH_REJOIN` arrives concurrently with an in-flight `startSyncMatch` that re-creates), the first room is silently dropped — its Wordle state, players, deadline are gone, and only the disconnect-grace timer keeps a reference. Also, on transactional retry inside `WithTransaction` callbacks, the room insert side-effect (`Add`) is not rolled back; if Mongo retries the callback, `startSyncMatch` is not re-run (it's outside the tx) so this is moot — but `Add` should still refuse to overwrite to fail loudly.
**Fix:** `func (r *RoomsRegistry) Add(matchID string, room *Room) error` returning `ErrAlreadyRegistered` when key exists. Caller treats as a fatal logic error.

### H4. `JoinAsChallengee` filter `{$exists: false}` plus app-level enforcement is inconsistent
**Severity:** High (correctness)
**Files:** `server/internal/store/matches.go:96-100`, `server/internal/store/models.go:65`
**Why it matters:** The filter checks `challengee_uid` does not exist. But `Match.ChallengeeUID *string ‵bson:"challengee_uid,omitempty"‵` — fine on insert, but if `Create` ever wrote an explicit `null` (e.g. through any future code path that doesn't go through `Create`), `$exists: true` would match and the join would silently fail with `ErrNoDocuments → ErrAlreadyJoined`. More importantly: there is **no DB-level uniqueness** on `challengee_uid` per match. The only thing preventing a duplicate join is the `WithTransaction`+`FindOneAndUpdate` round-trip. Under heavy concurrency on a sharded cluster (irrelevant for single-RS Atlas M0, OK now) this becomes a problem.
**Fix:** also add `state: "pending"` to the filter — defense in depth. Document explicitly that the partial-filter unique index on `share_token` is the enforcement boundary.

### H5. `cryptoSeed` calls `os.Exit(1)` — kills the entire server on a single rand.Read failure
**Severity:** High (availability)
**File:** `server/internal/ws/sync_match_handler.go:267-279`
**Why it matters:** `crypto/rand.Read` failure on Linux is essentially impossible (it reads from getrandom syscall, never blocks past boot), but **calling `os.Exit(1)` from a request handler is a denial-of-service amplifier**: if a kernel hiccup ever happens, every match-creation request takes the server down rather than failing one request. The alternative — mathematical randomness fallback — is rejected with the comment "predictable seeds break fairness". Correct — but failing this *one* request with a 500 is the right answer, not crashing the process. `log.Fatalf` would be marginally better but still wrong.
**Fix:** return `(0, error)` from `cryptoSeed`, propagate up, and respond 500 to the client.

### H6. `setActiveMatchID("")` on resolve, but the disconnect defer still races with resolution
**Severity:** High (incorrect forfeit on natural win)
**Files:** `server/internal/ws/match_room.go:246-248`, `server/internal/ws/conn.go:166-168`, `server/internal/ws/disconnect.go:32-61`
**Why it matters:** Sequence:
1. Player A makes the winning move; `HandleMove` calls `finishUnlocked` which calls `p.setActiveMatchID("")` and enqueues `MATCH_RESOLVED`.
2. The room is removed from the registry asynchronously (`go deps.Rooms.Remove(r.MatchID)`).
3. Player A's client receives the resolution, immediately closes the WS.
4. Conn disconnect defer fires; reads `getActiveMatchID()` — could see `""` (if step 1 wrote first) **or** the old matchID (if defer raced ahead).
5. If old matchID: `Schedule` reads `deps.Rooms.Get(matchID)` — could be nil (already removed) → no-op (correct), OR could still be present (the goroutine spawned at step 2 hasn't run yet), and the timer fires 30s later. If during those 30s the room IS removed, the timer's `Get` returns nil → no-op. But if a *new* match somehow uses the same matchID (won't happen, but)…

Practical worst case: player disconnects after winning but before `setActiveMatchID("")` is observable in their goroutine — defer schedules a grace timer for an already-resolved match. After 30s, `Rooms.Get` returns nil → no-op. **End result: harmless.** But the code path is fragile and an `Add`-then-`Remove` race could in principle re-resolve. The async `go deps.Rooms.Remove(r.MatchID)` at line 254 is a code smell — the comment "to avoid deadlock (registry lock ≠ room lock)" is wrong: `Remove` only takes the registry lock, and `finishUnlocked` holds the room lock. Calling `Remove` synchronously while holding `r.mu` is fine (no inverse path acquires `r.mu` while holding registry lock).
**Fix:** make `Rooms.Remove(r.MatchID)` synchronous in `finishUnlocked`. Removes one goroutine + one entire class of "registry not yet cleaned" races.

### H7. Solo Wordle session race: same-user across two open tabs corrupts state
**Severity:** High (correctness)
**Files:** `server/internal/ws/game_handler.go:55-138`
**Why it matters:** `sessions sync.Map` is keyed by `userID`. If a user opens two tabs (two WS conns, both authed as the same UID), both share the same `*wordleSession`. Per-session mutex serializes the writes, so no torn data — BUT each tab gets its own `WordleState` response based on the shared in-memory game. Sending guesses from tab A advances the state seen by tab B. This is "correct" in a sense (same daily, same state), but it's surprising — and `deleteSession(conn.userID)` in `conn.go:170` deletes the session as soon as **either** tab disconnects, dropping the other tab's in-progress game. Comment at `game_handler.go:111-114` claims "no user-visible data is lost" because the seed is deterministic — which is true, but the in-progress guesses ARE lost from memory until the next move re-loads (and re-loads to a fresh empty Wordle, since `loaded` is reset by the new `wordleSession{}` from `LoadOrStore`).
**Fix:** key sessions by `(userID, connID)` OR persist solo attempts to Mongo (already TODO in `game_handler.go:46`). For now, document the multi-tab failure mode in the package comment.

### H8. `Insert` (attempts) does a Find-then-Insert with no unique index → race opens a duplicate-attempt window
**Severity:** High (correctness, data integrity)
**Files:** `server/internal/store/attempts.go:53-74`, `server/internal/store/indexes.go:69-83`
**Why it matters:** The idempotency check at `attempts.go:55-65` reads, returns `ErrAttemptExists` if found, else inserts. Two concurrent `AttemptSubmit` calls for the same (matchID, playerUID) — possible if a client retries a slow request — both pass the check, both insert. The compound index at `indexes.go:78` is **not unique**. Net: two attempt docs for the same player in the same match. `decideWinner` then sees inconsistent guess counts. `WithTransaction` does NOT save you here unless the index is unique (transactions on a replica set are MVCC — they don't serialize on a non-unique compound).
**Fix:** make the `(match_id, player_uid)` compound index unique. Drop and recreate is fine — empty in dev. Then handle `mongo.WriteException` with code 11000 → return `ErrAttemptExists`. Belt and suspenders: keep the in-tx Find as well for the fast path.

### H9. `IncrementStats` runs inside the `WithTransaction` callback — slow path, retries do double-increment if the user-update succeeds but the tx aborts
**Severity:** High (data integrity)
**Files:** `server/internal/ws/match_handler.go:236-240`
**Why it matters:** `WithTransaction` retries the callback on transient errors. Both `MatchRepo.Complete` and `UserRepo.IncrementStats` run inside it. On a retry that races with another `Complete` already winning (because the filter you have on `Complete` is `{_id: oid}` not `{_id, state: "active"}` — see L1 below), the callback can re-run `IncrementStats` after the previous attempt's increments already committed. `WithTransaction`'s retry semantics roll back the *transactional writes* but don't undo previously-committed state. Result: under contention, win/loss counts can be incremented twice.

`CompleteSync` (sync match) at `matches.go:198` correctly filters on `state: "active"`, but `Complete` (async match) at `matches.go:136` does NOT. So async PvP is the vulnerable path.
**Fix:** add `state: "pending"` filter to `Complete`. Make `IncrementStats` idempotent by checking `match.state` post-update (use `FindOneAndUpdate` with `state: pending → complete` and only increment when ModifiedCount==1).

### H10. Server logs leak Firebase UIDs at INFO level on every connection failure
**Severity:** Medium-High (PII/data hygiene)
**Files:** `server/internal/ws/conn.go:129`, `server/internal/ws/match_handler.go:99,115,238,245`, `server/internal/ws/match_room.go:217,224`, etc.
**Why it matters:** Firebase UIDs are not directly PII but they are stable user identifiers. They appear in dozens of `log.Printf` calls. In a SOC/compliance review this is a finding. More important: the `ws upsert user` log line on conn.go:129 fires on every connect — anyone with stdout/journalctl access has a connection-rate user activity stream.
**Fix:** centralize a `logEvent(level, fields...)` helper that hashes UIDs (HMAC-SHA256 with a per-deploy salt) before emit. At minimum, rate-limit the upsert-fail log.

---

## Medium-impact issues

### M1. `Attempt.Hints` is on the model but never set
**File:** `server/internal/store/models.go:83`, `server/internal/ws/match_handler.go:176-183`
The struct field exists with `bson:"hints,omitempty"`, but no code path populates it. Dead field or missing functionality. If unused, drop the field; if intended for replay/anti-cheat audits, populate from `WordleMove.GetGuess()` rerun via `wordle.Score`.

### M2. `LeaderboardRepo.Refresh` re-fetches all attempts of all matches every 5 minutes — O(N) growth, no incremental refresh
**File:** `server/internal/store/leaderboards.go:48-192`, `server/internal/scheduler/tick.go:67`
The comment acknowledges the trade-off ("<10K matches/day") but in steady state at 10K matches × 2 attempts = 20K docs decoded into Go memory every 5 min. With `cur.All` this is one O(N) memory spike. Better: aggregation pipeline `$lookup` + `$sort` + `$limit 100`, or keep an incremental "dirty set" of UIDs to recompute.

### M3. `parseDBName` ignores URI parse errors silently
**File:** `server/internal/store/mongo.go:76-82`
A malformed `MONGO_URI` returns empty path → defaults to "dleague". Fine for dev, surprising in prod where the URI may include a non-default DB. Boot should fail rather than silently fall back.

### M4. `health` endpoint hits Mongo on every probe
**File:** `server/internal/http/health.go:27-37`
If a load balancer probes once per second, this is an extra 86,400 pings/day. On Atlas M0 with 500-conn cap, harmless. On larger plans, prefer cached liveness (`atomic.Bool` flipped by a slower background ping). Lower urgency.

### M5. `EvictExpired` notifies clients while holding `q.mu` — `notify` calls `c.EnqueueError` → `c.enqueue` → `proto.Marshal` + channel send. If the conn's send buffer is full, `cancelRead()` is called inside the notify path. Cancel itself does not block — fine. But marshalling under a contended lock is wasteful.
**File:** `server/internal/ws/queue.go:64-85`
**Fix:** collect evicted conns under lock, release lock, then notify.

### M6. `OriginPatterns` is a coder/websocket pattern matcher (glob-style), not a strict equality list
**File:** `server/internal/ws/conn.go:81`, `server/internal/config/config.go:90`
`DLEAGUE_WS_ORIGINS` is documented as "host:port matched case-insensitively" but `OriginPatterns` accepts wildcards (`*.example.com`). Misconfiguration could open broader access than intended. Verify pattern docs match operator expectations; consider parsing+rejecting wildcards if not desired.

### M7. Rate limiter is per-conn; no per-UID or per-IP limit
**File:** `server/internal/ws/rate_limiter.go`
A determined attacker opens 100 connections, each gets its own bucket → 1000 messages/sec. Combined with `MaxConns=1000` default, the upper bound is generous. Add a per-UID bucket (sync.Map keyed by userID) and/or per-IP bucket at the upgrade layer. Nice-to-have for the MVP.

### M8. No request size limit on Mongo writes besides the 1 MiB WS read limit
**File:** `server/internal/ws/conn.go:21`
`MatchMove.guess` is bounded by Wordle to 5 chars; `AttemptSubmit.guesses` is unbounded — a client could submit a 100K-element guesses array and pay only the WS frame cost. Validate `len(msg.GetGuesses()) <= MaxAttempts` (6) before persisting.

### M9. `wordle.contains` is O(n) linear over a 864-word dictionary — acceptable now, but called on every guess
**File:** `server/internal/game/wordle/wordle.go:124-131`
Build a `map[string]struct{}` once at boot, store on `Wordle` or as a package-level cache. ~10x speedup on validation. Already noted as Phase 10 work in the comment.

### M10. `mustParseDate` can panic on bad input from `nowUTC().Format` consumers
**File:** `server/internal/store/leaderboards.go:51,253`
Currently safe (date is always `time.Format("2006-01-02")`). But the public `Refresh` accepts an arbitrary `date` string — if a future caller passes a bad value, the scheduler goroutine panics and is recovered only by the caller's `defer`. Scheduler's `Run` has no `recover` (`scheduler/tick.go:49-83`). Return error instead of panic.

### M11. `validateSyncGuess` constructs a temporary `wordle.New("CRANE")` for every move
**File:** `server/internal/ws/sync_match_handler.go:258-262`
Allocates a struct per move just to call `Validate`. Make `Validate` a package-level function: `wordle.ValidateGuess(guess string, dict []string) error`.

### M12. `match_handler.go` is 342 lines — exceeds 200-line guideline
**File:** `server/internal/ws/match_handler.go`
Split: challenge (create/join), attempt submit, helpers. Same applies to `conn.go` (312) and `match_room.go` (268).

### M13. No structured observability (metrics)
The codebase uses `log.Printf` exclusively. No counters for matches created, attempts/sec, errors, or conn-count gauges. For an MVP serving real users, even `expvar` would catch capacity issues earlier than tail-latency probes.

### M14. Test coverage gaps
- **No tests for `match_handler.go`** (challenge create/join/submit). The most security-sensitive Mongo-tx paths are uncovered.
- **No tests for `leaderboard_handler.go`, `game_handler.go`, `sync_match_handler.go`.**
- **No tests for `store/matches.go` `JoinAsChallengee` race semantics** (would need a docker-compose Mongo replica set). At minimum, a unit test driving two goroutines through `JoinAsChallengee` against a real `mongod --replSet`.
- **No fuzz tests for `proto.Unmarshal` paths.** Worth adding `go test -fuzz` on `extractFirebaseToken` and the envelope dispatcher.
- Existing tests are good (`hub_test.go`, `match_room_test.go`, `queue_test.go`, `rate_limiter_test.go`, `disconnect_test.go`) — well-targeted, race-aware. The bar is set; uncovered files stand out.

### M15. `decideWinner` perfect-tie favors challenger — surprising default
**File:** `server/internal/ws/match_handler.go:323`
`return match.ChallengerUID // perfect tie → challenger wins` is undocumented to clients. If the spec intends ties to be "no winner" rather than "challenger wins", the wire schema needs an `is_tie` bool. Confirm with product.

---

## Low-impact / nits

### L1. `Complete` (async) doesn't filter on state
**File:** `server/internal/store/matches.go:136`
`UpdateOne(bson.M{"_id": oid}, ...)` — should be `{_id: oid, state: "pending"}` to mirror `CompleteSync`'s defensive filter. Pairs with H9.

### L2. `OriginPatterns` empty-slice semantics
**File:** `server/internal/ws/conn.go:79-81`
The comment at `conn.go:62-65` says "Zero value enforces the coder/websocket default same-origin policy." Verified by reading the lib: yes, empty `OriginPatterns` triggers same-origin-only. Good — but document that fact in `config.go` next to `AllowedOrigins`.

### L3. `parseWordList` silently drops malformed words
**File:** `server/internal/game/wordle/wordlist.go:66-76`
Drops blank lines (good) but also non-5-letter lines without warning. A typo'd source file would yield a partial dictionary at boot with no signal. Log dropped count.

### L4. `errorEnvelope` swallows marshal errors
**File:** `server/internal/ws/error.go:14-20`
Comment explains the trade-off; acceptable. But on a real `proto.Marshal` failure, `body` is nil and the client gets an envelope with an empty error body. Add a `log.Printf` for observability.

### L5. `decodeServiceAccount` writes to `/tmp/dleague-sa.json` with mode 0600 — fine on a single-tenant container. Not fine on a shared host. Document this assumption explicitly.
**File:** `server/cmd/api/main.go:36-53`

### L6. `seed-wordlists/main.go` uses `DLEAGUE_MONGO_URI` while `config.go` uses `MONGO_URI`. Two different env names for the same value.
**Files:** `server/cmd/seed-wordlists/main.go:24`, `server/internal/config/config.go:91`

### L7. `match_handler.go:150` — `tms < 500 || tms > 86_400_000` is a 422 response, but this is a *cheating attempt* signal, not a validation error. Worth logging at warn level with the UID.

### L8. `displayName(c *Conn)` returns `c.userID` as fallback — leaks UIDs into broadcast messages (`QueueMatched.OpponentDisplayName`).
**File:** `server/internal/ws/sync_match_handler.go:247-255`
The comment acknowledges this is MVP. But shipping with UIDs in opponent display is a privacy regression. Better fallback: "Player ${last4(uid)}" or "Anonymous".

### L9. `wordle.Wordle.ToProto` makes defensive copies of `guesses` and `hints` — good. But `colors := make([]Color, len(h)); copy(colors, h)` then assigns to a fresh `WordleHint{Colors: colors}` is fine; `copy([]string(nil), w.guesses...)` allocates two slices when one would do.
**File:** `server/internal/game/wordle/wordle.go:101-120`
Micro-perf only.

### L10. Comment-vs-code drift
- `hub.go:44` claims `NewHub` initializes "Queue, Rooms, GraceTimers" — it does not; `main.go` does.
- `attempts.go:53-65` claims the idempotency check participates in the transaction. It does, but only if the caller passes the session-carrying ctx. Code does, but the comment overstates the API guarantee.

### L11. `NewSessionContext` removed in v2 driver — code is correct, but `matches.go:1-7` and `attempts.go:1-5` package comments restate the v2 migration in detail. Consolidate to one place.

### L12. `currentSchemaVersion = 1` constant in `models.go:11` — the lazy migration check ("Option A from research §6") is never implemented in any read path.

### L13. `game_handler.go:55` comment mentions `sync.Map` "chosen for concurrent access without a coarse hub-level lock" — fine, but `sessions` is a global mutable singleton. Test isolation requires a hook to reset; tests currently rely on uniqueness of UIDs.

### L14. `wordle.go:124` `contains` is exported in concept but lower-case → fine; just noting it's duplicated by callers in tests rather than reused.

---

## Strengths

- `go test -race` clean assumption is plausible: explicit mutex discipline in `Hub`, `RoomsRegistry`, `Queue`, `RateLimiter`, `GraceTimers`, `Conn.mu`, `wordleSession.mu`. Phase fixes documented inline (M3, M5, M6 callouts).
- Server-authoritative Wordle (`wordle.go`) and color algorithm (`colors.go`) are clean and correct — two-pass scoring handles repeated letters properly. Tests are thorough (`colors_test.go`, `wordle_test.go`).
- Sound transactional discipline: `JoinAsChallengee` uses `FindOneAndUpdate` with `$exists:false`; `CompleteSync` filters on `state:"active"`; idempotent `AttemptSubmit` retry path lifts result from `match.State == "complete"`.
- Production-safety boots: explicit `cfg.IsProduction() && len(AllowedOrigins) == 0` fail-fast, SRV-URI warning, fixed-mode service-account decode.
- Rate-limit on overflow drops the frame (429) rather than killing the conn — correct trade-off for a token bucket.
- Sentinel errors (`ErrAlreadyJoined`, `ErrAttemptExists`, `ErrAtCapacity`, `ErrEmptyUID`, `ErrMissingProjectID`, `ErrWrongLength`, `ErrNotInDictionary`, `ErrGameOver`) — clean error contracts.
- Read limits (`readLimit = 1 MiB`), `request_id` cap (128 bytes), `MaxConns` and pre-accept count check — defensive layering.
- HTTP-server timeouts set explicitly (`ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`) — Slowloris-resistant.
- CSP/X-Frame-Options/Referrer-Policy applied to static assets and intentionally NOT to /ws — correct.
- Auth-refresh forbids UID switch mid-connection (`auth_refresh.go:42-46`) — closes a session-pivot vector.
- Anti-cheat for `time_ms` range, deterministic daily seed via `sha256`, solution withheld until terminal (`wordle.go:116-118`).
- Pluggable game registry (`shared/game/registry.go`) is minimal and testable; `Wordle` uses it correctly via `Result()`.
- `EnsureToday` is idempotent at boot AND per-request — robust against partial restarts.
- CompletedAt/state filters on match resolution to prevent double-resolve.

---

## Unresolved questions

1. **Atlas tier**: `mongo.go:23-24` claims "Atlas M0 (max 500 conns)". Confirm the actual prod tier; pool sizes (`100 max`, `10 min`) may be too aggressive for M0 (which throttles at 500 cluster-wide and 100 per Atlas user).
2. **Tie-break policy**: should perfect-tie in async PvP favor challenger (current behavior, `match_handler.go:323`) or be an explicit tie? Affects wire schema.
3. **Single-region assumption**: comments say "Single-region MVP; Redis pub/sub deferred to v2" (`queue.go:16`). Is horizontal scale-out a near-term goal? If yes, the in-memory queue/rooms become a P0 to redesign — affects how aggressively we should rewrite this code now.
4. **Shutdown ordering**: `main.go:221-227` calls `hub.CloseAll` then `srv.Shutdown`. If a WS conn is mid-`HandleMove` writing to Mongo when `CloseAll` cancels its read, does that move complete? (Currently: ctx cancel propagates into `MatchRepo.CreateSync` etc.) Acceptable, but document that in-flight writes are lost on SIGTERM if they haven't acked.
5. **Solo session multi-tab semantics** (H7): is a single-session-per-UID lock the desired UX, or should each tab get an independent game?
6. **Are server logs going to a SIEM?** If yes, H10 is a compliance must-fix; if not, it's a hygiene issue.
7. **`bson.NewObjectID` collision tolerance**: is the 96-bit ObjectID collision rate (effectively zero at <100K matches/day) considered "good enough" for the matchID space, or should we require an explicit uniqueness check on insert?

---

**Status:** DONE_WITH_CONCERNS
**Summary:** Code is well-structured with disciplined concurrency primitives and good documentation, but ships with several production correctness gaps: queue cleanup on disconnect (H1), unguarded conn-field reads cross-goroutine (H2), unique-index gap on attempts (H8), and double-counted stats on tx retry (H9) are the must-fix items before ship. ~14 medium issues and many nits — most are tractable in 1–2 days of focused cleanup.
