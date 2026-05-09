# Architecture / contract / docs / tooling review

Scope: protobuf schema, `shared/game` contract, module layout, docs accuracy,
code-standards self-compliance, test/CI infra, repo hygiene, plans, roadmap.
Skipped per request: Dockerfile, fly.toml, docker-compose.yml,
deployment-guide.md, scripts/set-fly-secrets.sh.

---

## High-impact issues

### H1 — `shared/game.Game` is theater; nothing implements or registers it

`shared/game/game.go:47-67` defines a `Game` interface with `Init / Validate /
Apply / IsTerminal / Result`. `shared/game/registry.go` exposes `Register / New
/ IDs`. **Neither is called anywhere in the codebase.**

- Grep for `game.Register(`: 0 hits in `server/`, 0 in `web/`.
- `wordle.Wordle` does not satisfy `game.Game`:
  - `Wordle.Validate(guess string, dict []string) error` (concrete signature) vs
    interface `Validate(move Move) error`.
  - No `Init(seed int64) error` method exists on `*Wordle`.
- `server/internal/ws/game_handler.go:88`, `match_room.go:39`, etc. construct
  `wordle.New(solution)` directly — bypassing the registry entirely.
- `dispatch` in `hub.go` switches on `MESSAGE_TYPE_GAME_MOVE` and calls
  `handleGameMove`, which is hardcoded to wordle (`*wordle.Wordle`,
  `wordle.EnsureToday`, `wordle.New`). Nothing routes by `gameID`.
- `server/internal/store/games.go:NewGameRepo` is constructed and discarded:
  `_ = store.NewGameRepo(db)` at `server/cmd/api/main.go:95`. It also has
  `TODO(phase-07): RegisterGame / ListGames` markers despite Phase 07 being
  marked completed.

**Why it matters:** PDR + README + roadmap all promise pluggable game types
("music, geography, image variants planned"). With the current shape, adding
"musicdle" requires editing `dispatch`, `GameDeps`, `WordleMove/State` proto
messages (or duplicating them), and the wordle-specific session map — i.e.,
inventing the abstraction during the second game's implementation, not before.
The interface as written is also wrong for that abstraction (Move is `interface{}`
with no marshal/unmarshal contract; `WordleMove` proto is referenced
directly inside handlers, which won't generalize).

**Concrete fixes** (one of):
1. Delete `shared/game.Game` interface + `Registry` + dead `GameRepo` and stop
   advertising "pluggable" in PDR/README until a second game is real (YAGNI).
2. Make wordle implement the interface, register it in `init()`, route by
   `gameID` in `dispatch`, and define the contract for proto-typed Move/State
   (e.g., `Game` exposes `MoveSchema`, `StateSchema`, or returns proto reflect
   descriptors). The current interface using `Move interface{}` cannot survive
   wire-format coupling.

Either way the PDR's claim "shared/game.Game interface supports Wordle at
launch" (project-overview-pdr.md:17) is presently untrue.

### H2 — Envelope uses `bytes payload` rather than `oneof` — extensibility cost

`proto/dleague/v1/envelope.proto:39-43`:

```proto
message Envelope {
  MessageType type = 1;
  string request_id = 2;
  bytes payload = 3;
}
```

Two-stage marshalling: every handler does `proto.Unmarshal(env.GetPayload(),
&inner)`. Trade-offs vs `oneof Payload`:

- Pro: stable wire shape across new message types; doesn't bloat the
  generated `Envelope` struct as game variants grow.
- Con: type/payload pair can desynchronize at the wire (a malformed peer can
  send `type=GAME_MOVE` with a `WordleState` payload and the server only
  detects it when the inner Unmarshal happens to succeed silently — proto
  Unmarshal is lenient about extra/missing fields).
- Con: `MessageType` enum is a single flat namespace per game family; today
  18 wordle/match types share it, and the PDR plans music/geography/image.
  The numeric range `0-23` is already used for envelope+wordle+match; adding
  a per-game family uses 8+ slots each. With `oneof Payload` the server gets
  exhaustiveness checking from generated code, and per-game proto files own
  their type space.
- Con: no field reuse hint in `MessageType` (e.g., `MESSAGE_TYPE_GAME_MOVE`
  stays even though new games would want `MESSAGE_TYPE_<GAME>_MOVE`).
  Currently `GAME_MOVE` carries `WordleMove` — locking the value 6 to wordle
  semantics permanently.

**Recommendation:** Either accept the `bytes payload` design as final and
document this constraint in code-standards.md, or migrate to a `oneof
Payload` while only one game ships. Mixing is the worst path.

### H3 — Backwards-compat hazard: `MESSAGE_TYPE_GAME_MOVE/STATE` are wordle-shaped

Comment in `envelope.proto:15-16` calls the values `GAME_MOVE` / `GAME_STATE`
(generic) but every server/client codepath unmarshals the payload as
`WordleMove` / `WordleState` (wordle-specific). When a second game ships:

- Either the values 6/7 are repurposed (breaking change; `buf breaking` will
  flag it because deployed clients still expect `WordleMove` semantics), or
- New `<GAME>_MOVE/STATE` enum values are added, and the existing 6/7 stay
  forever as wordle-only. That commits to the type-cluttering pattern noted
  in H2.

Fix path: add a `game_id` field to the move/state envelopes so the inner
schema can be selected at dispatch time, OR rename `GAME_MOVE` →
`WORDLE_MOVE` now while `buf breaking` will not flag it (no live clients).

### H4 — Docs/code drift: `system-architecture.md` dispatch table is wrong

`docs/system-architecture.md:118-121` lists message types that don't exist:

```
| CREATE_MATCH    | handleCreateMatch    | yes |
| JOIN_MATCH      | handleJoinMatch      | yes |
| SUBMIT_ATTEMPT  | handleSubmitAttempt  | yes |
| MATCH_RESULT    | handleMatchResult    | yes |
```

Actual proto + dispatch use `CHALLENGE_CREATE / CHALLENGE_JOIN /
ATTEMPT_SUBMIT` (see `envelope.proto:18-23`, `hub.go:119-138`). There is no
`MATCH_RESULT` enum or handler at all. New contributors reading the
architecture doc will write code against names that don't exist.

### H5 — `match_handler.go:115` logs share token (sensitive credential)

```
log.Printf("ws match_handler: JoinAsChallengee token=%q uid=%q: %v",
           token, c.userID, txErr)
```

`token` is the bearer credential for the challenge link — it identifies the
match and is the only secret keeping the challenge from being joined by
anyone with the URL. Logging it to stdout means anyone with Fly log access
(or a future log drain) can join challenges by replay. Hash or truncate
before logging (`token[:8]+"…"`).

### H6 — Repomix snapshot is gitignored but lying around (518 KB)

`repomix-output.xml` (518 KB) sits in repo root, is in `.gitignore`
(`/.gitignore:38`), and confirmed not tracked by git. **However** it is
visible to anyone with filesystem access to the worktree — including
backup tools, IDE indexers, and any subagents that grep the tree. It
contains a complete code+config snapshot that may include `.env.example`
contents and other config. Either:

- Move generated repomix outputs to `plans/<active>/visuals/cache/` (already
  gitignored at `plans/visuals/cache/`), or
- Run `make clean` to delete it, or
- Add a Makefile target `repomix-clean` and document it.

Not strictly a bug, but a stale 518 KB context dump persisting in worktrees
will rot fast.

---

## Medium-impact issues

### M1 — Code-standards self-compliance: 8 Go files exceed 200 LOC limit

`docs/code-standards.md:8` says **"Max 200 LOC per file — Split early"**.
Actual file sizes (counted):

| File | LOC |
|------|-----|
| `server/internal/ws/match_handler.go` | 342 |
| `server/internal/ws/conn.go` | 312 |
| `server/internal/ws/match_room_test.go` | 299 |
| `server/internal/ws/sync_match_handler.go` | 279 |
| `server/internal/ws/match_room.go` | 268 |
| `server/internal/ws/conn_test.go` | 263 |
| `server/internal/store/leaderboards.go` | 259 |
| `server/internal/store/matches.go` | 227 |

Web side: `web/src/routes/play/+page.svelte` 345 LOC, `web/src/lib/ws.ts`
364 LOC, `web/src/lib/components/sync-game-scene.svelte` 233 LOC also
exceed (`code-standards.md:159` says <200 for Svelte components too).
Test files are excluded by some conventions but the doc doesn't carve them
out. Either relax the rule (with rationale: "200 LOC is a guideline, not
hard cap; tests + handlers with many switch cases may exceed") or refactor.

### M2 — `Dockerfile` uses `golang:1.24-alpine` but `go.mod` declares `go 1.26`

`Dockerfile:10` pins the build image to Go 1.24 alpine. `server/go.mod:3`
and `shared/go.mod:3` and `go.work:1` all declare `go 1.26`. CI uses `1.26`
(`.github/workflows/ci.yml:26`). The Dockerfile relies on `GOTOOLCHAIN=auto`
to download 1.26 at build (commented line 20). This works but:
- Reproducibility: the toolchain download adds ~50 MB and a network
  dependency at build time even with cached base layers.
- Drift risk: when 1.26 reaches GA in golang official images, switch the
  base image to `golang:1.26-alpine`. Until then, leave as-is but document.

### M3 — Go 1.26 may not be released yet (consistency check)

Knowledge cutoff is Jan 2026; Go 1.26 likely targets Aug 2026 per the
typical release cadence. If Go 1.26 isn't actually GA at review time, the
toolchain auto-download in Dockerfile may fail. Verify against `go version`
locally before deploy.

### M4 — `daily.go` seeding has a tiny bias

`daily.go:58`: `idx := seed % int64(len(answers))`. With 772 answers and
seed in `[0, 2^63-1]`, the distribution has a modulo bias (some words are
~1 in 2^63 / 772 more likely than others). Practically zero impact for a
daily puzzle, but worth noting if a future review flags it. Same pattern
in `sync_match_handler.go:185` (sync-match seed → answer index).

### M5 — Daily puzzle can repeat across day boundaries

`daily.go:53`: seed = `sha256(date + "wordle-v1")`. With 772 answers,
~365 days produces a collision probability of ~50% within a year (birthday
problem: √(2·772) ≈ 39 days median for first repeat). The PDR claim
"Everyone plays the same puzzle each day" is satisfied, but users will
notice the same word recurring within weeks. Either accept (Wordle does
this too pre-NYT-acquisition) or seed by `(date, lastN_seeds_seen)` to
guarantee non-repetition for some window. Roadmap already says "real
2315-word list" lands in v2 which makes the collision math much friendlier.

### M6 — `tokenToProfile` accepts `claims map[string]interface{}` — type unsafe

`conn.go:194`: `func tokenToProfile(claims map[string]interface{})`.
Internally does `claims["name"].(string)` etc. The Firebase Go SDK exposes
typed accessors via `auth.Token.Firebase` — and `isAnonymousToken`
(`auth_refresh.go:69`) already uses `token.Firebase.SignInProvider`
correctly. Pass the typed `*auth.Token` through and use typed accessors.

### M7 — `auth_refresh.go:49` mutates `c.userID` without holding `c.mu`

```go
c.userID = token.UID
c.tokenExpiresAt = time.Unix(token.Expires, 0)
c.isAnonymous = isAnonymousToken(token)
```

`c.userID` is read from many goroutines (disconnect defer in `conn.go:170`,
match-room timer in `disconnect.go:56-57`, leaderboard handler, etc.). The
read loop is single-threaded, so the write/read pair within one connection
is sequenced — but the disconnect timer goroutine reads `c.userID` after
`time.AfterFunc` fires, which can race with a `handleAuthRefresh` writing
it. Add `c.mu.Lock()` around the three-field write or use atomic.Value.
The same holds for `auth_refresh` updating fields after the conn is in an
active match: a concurrent grace-timer goroutine could read inconsistent
state.

(Note: the comment on `Conn.mu` at `conn.go:42-44` already mentions the
mutex is for `activeMatchID`; documentation says nothing about other auth
fields. Either extend the mutex's scope or stop mutating those fields
post-upgrade.)

### M8 — `Wordle` interface contract is leaky

`shared/game/game.go:31`: `type Move interface{}`. Empty interface with no
methods means callers must type-assert. The intent (proto-marshalable
move) is unstated. Either:
- Define `type Move interface { ProtoReflect() protoreflect.Message }` so
  every Move is wire-marshalable, or
- Define a `MoveCodec` per game registered alongside the factory.

Either approach unblocks the handler-side dispatch refactor blocked in H1.

### M9 — `repomix-output.xml` is 518 KB sitting in the worktree

Already covered in H6. Lower severity than the lying-around aspect: the
file isn't committed but persists if the user forgets to clean it.
Recommendation: gitignore matches; consider auto-clean as part of `make
clean`.

### M10 — `plans/journals/` has 1 file; convention suggests more is intended

`plans/journals/260505-1127-phase-01-foundation-shipped.md` is the only
journal. If the intent is "one journal per phase ship", phases 02-10 are
missing. Either backfill or remove the directory and adjust convention.

### M11 — Generated TS protobuf size: 476 LOC for `match_pb.ts`

`web/src/lib/pb/dleague/v1/match_pb.ts` is 476 LOC, well above the 200 LOC
component rule. Rule should explicitly carve out generated code (Go-side
already does via `.golangci.yml` exclusions; TS rule doesn't).

### M12 — `protoc-gen-es` path in `buf.gen.yaml` requires npm install pre-step

`proto/buf.gen.yaml:13`: `local: ../web/node_modules/.bin/protoc-gen-es`.
This couples `make proto-gen` to "user already ran `npm ci` in `web/`".
Makefile mentions this in the `proto-gen` target description but doesn't
enforce it. Either add `web-install` as a prerequisite of `proto-gen`, or
gate with a check + helpful error message.

---

## Low-impact / nits

### L1 — README is mildly inconsistent with itself

`README.md:56` says "Repo layout (planned)" — the layout shown matches the
actual repo, so drop "(planned)".

### L2 — README quickstart references `make compose-up` for Mongo + Firebase emulator separately, but `make dev` only runs server

The "boots Mongo + emulator + Go server + SvelteKit dev server cleanly"
success criterion in `plans/.../plan.md:93` isn't satisfied by any single
target in the `Makefile`. Consider an `all-up` target.

### L3 — `Makefile:9` `.PHONY` list mentions `deploy-staging` (kept) but excludes `seed-wordlists-prod` (line 93)

Add `seed-wordlists-prod` to `.PHONY` for consistency.

### L4 — Comment lies: `wordle.go:122` says "Phase 10 can optimise"

Phase 10 is marked complete; comment is stale ("Dictionaries are small
enough (<5000 words) that a map is unnecessary for MVP; Phase 10 can
optimise"). Either delete the second sentence or backfill an issue.

### L5 — `wordlist.go:1-7` has a `TODO(phase-10)` block

Phase 10 is closed; TODO references the closed phase. The unresolved item
("real 2315-word list, license verification") is a real v2 backlog item —
move it to `development-roadmap.md` (already there as "high priority") and
delete the in-code TODO to avoid duplication.

### L6 — `store/games.go:39-41` has unimplemented TODOs but Phase 07 is closed

```
// TODO(phase-07): RegisterGame — insert or replace a game registry entry.
// TODO(phase-07): ListGames — return all active games.
```

Pick one: implement, or delete `GameRepo` entirely (see H1).

### L7 — `system-architecture.md:90` says "Go 1.23"

`go.mod` says `go 1.26`. Drift from earlier doc revision; one-line fix.

### L8 — `codebase-summary.md:61-63` describes a `Game` interface that doesn't match `shared/game.Game`

Doc says "`game.Game` — pluggable game interface (`Validate`, `Apply`,
`ToProto`, `Score`)". Actual interface is `Init/Validate/Apply/IsTerminal/
Result`. `ToProto` and `Score` are not part of the interface (they're
methods on concrete `Wordle`).

### L9 — `system-architecture.md:5` says "skeleton — diagrams + ERD landed by Phase 10" (status line)

Phase 10 is closed; the status line should say "current" or be removed.

### L10 — README:99 references `plans/archive/README.md` link with relative path; broken from web view

Cosmetic — on `gh repo` web view the link works, just noting.

### L11 — `.env.example:35` mentions emulator at `127.0.0.1:9099`; firebase.json should match

`firebase.json` exists (91 bytes); haven't verified its emulator port. If
it differs from 9099, dev experience breaks.

### L12 — `match_handler.go:148-150` magic numbers

```go
if tms < 500 || tms > 86_400_000 {
    return errorEnvelope(env.GetRequestId(), 422, "time_ms out of range [500, 86400000]"), nil
}
```

Extract `minTimeMs = 500` / `maxTimeMs = 24*time.Hour.Milliseconds()` as
named constants.

### L13 — `MESSAGE_TYPE_QUEUE_LEAVE = 17` has no ack but the server returns nil silently — test for it

`hub.go:147-149` returns `nil, nil` when `GameDeps == nil`. Without an ack,
the client can't tell whether leave succeeded. Acceptable behavior, but
should be documented in proto comment.

### L14 — `cryptoSeed` in `sync_match_handler.go:267` calls `os.Exit(1)` — surprising in a library function

Comment says "fail-loud", which is reasonable for crypto/rand failure, but
calling `os.Exit` from a deeply-nested handler is non-idiomatic Go. Prefer
`log.Fatalf` (does the same thing but is more searchable).

---

## Strengths

- **Phase discipline**: Plans → reports → changelog flow is consistent and
  followable. Archived plans have a README explaining each cancellation.
- **Dependabot + SHA-pinning**: `.github/workflows/ci.yml` uses immutable
  SHAs for all `uses:` lines and weekly Dependabot is configured. Modern
  hygiene that many repos skip.
- **Server-authoritative game design**: `WordleState.solution` is genuinely
  hidden until terminal (`wordle.go:115-118`); test coverage exists
  (`TestToProto_SolutionHiddenPreTerminal`). Letters-never-leak invariant
  in sync match has byte-level test.
- **Atomic match-end**: `JoinAsChallengee` and `CompleteSync` use Mongo
  transactions correctly; `state:"active"` filter prevents double-resolve.
- **Rate limiting + connection cap + origin allowlist** are real and tested,
  not placeholder.
- **Idempotent repository writes**: `EnsureToday`, `EnsureIndexes`,
  `UpsertByUID`, `CompleteSync` are all idempotent which makes restarts
  safe.
- **Good error wrapping**: `fmt.Errorf("...: %w", err)` is consistent
  across `store/`.
- **Test infra**: Mongo service in CI, Firebase emulator started in CI, race
  detector enabled. `MONGO_TEST_URI` and `FIREBASE_AUTH_EMULATOR_HOST`
  documented.

---

## Suggested additions to roadmap

- **Pluggable-game contract**: Either delete `shared/game.Game` (YAGNI) or
  wire wordle through it before promising music/geography. Decision needed.
- **Proto schema cleanup**: rename `MESSAGE_TYPE_GAME_MOVE/STATE` to
  `WORDLE_MOVE/STATE` while no clients are deployed; or commit to
  `oneof Payload` envelope.
- **Doc fact-check pass**: One reviewer rereads `system-architecture.md`
  (stale dispatch table, Go version, status header) and `codebase-summary.md`
  (interface signature) against grep.
- **Refactor handler files** (>200 LOC) per code-standards self-compliance,
  or update standard with a documented carve-out for handlers/tests.
- **Token logging audit**: add a lint rule (golangci-lint custom or
  semgrep) banning `log.Print*` of any var named `token`/`shareToken`.
- **Stress / load test**: roadmap mentions "200-connection load test" as
  exploratory — promote to medium given the WS hub is central.
- **Structured logging (slog)**: roadmap has it; bump priority for cleaner
  log redaction / filtering.
- **TS bundle splitting**: `match_pb.ts` 476 LOC ships in main bundle;
  consider lazy-loading per-route.
- **Integration test for full match lifecycle**: queue→pair→move→resolve
  end-to-end, in-process, asserting no goroutine leaks. Not present today.
- **`make all-up` target**: spawn Mongo + Firebase emulator + server + web
  dev all together for one-command DX.
- **Clean up dead code**: remove `_ = store.NewGameRepo(db)`, the
  `KeyEnter/KeyBackspace` constants in `shared/game/game.go:38-40` (kept
  "for compatibility with interactive key-driven games" — comment says
  WS-based games use Move; constants are unused).

---

## Unresolved questions

1. **Is "pluggable game types" truly a v2 commitment, or is it advertising
   that should be downgraded to "wordle-only"?** Roadmap places music/
   geography in "lower priority / exploratory". README and PDR speak in
   present tense ("supports Wordle at launch"). The interface is currently
   non-functional, so the messaging needs to match reality.
2. **Should `MESSAGE_TYPE_GAME_MOVE` be renamed before any client ships?**
   `buf breaking` against `main` will not flag a one-time rename if no
   external consumers exist yet, and saves a lot of pain later.
3. **Code-standards 200-LOC rule: hard cap or guideline?** Currently 8
   server files exceed it; either standard relaxes (with documented carve-
   outs for handlers, tests, generated code) or refactor sprint is owed.
4. **Is `repomix-output.xml` an artifact that should be auto-cleaned by
   `make clean`?** Currently it lingers and gets stale.
5. **Daily-puzzle answer collisions** within a year are mathematically
   likely with 772 answers — is this acceptable for MVP, or should a
   bloom-filter / rolling exclusion be added before launch? Roadmap defers
   to "real 2315-word list" which mostly fixes it.
6. **Plan `260508-2300` shows `status: completed` but `Success Criteria`
   block uses unchecked `[ ]` boxes (lines 93-103).** Are these literal
   unchecked or just a stylistic choice? If literal, plan status conflicts
   with phase status table.

---

**Status:** DONE_WITH_CONCERNS
**Summary:** Reviewed 25+ files across proto, shared, server, web, docs, plans,
CI. Two structurally significant findings: (1) `shared/game.Game` interface +
registry are dead code that contradict the "pluggable" pitch in PDR/README,
and (2) `system-architecture.md` dispatch table names message types that don't
exist. Several smaller doc/code drift items, one credential-logging bug
(share token at `match_handler.go:115`), and code-standards self-compliance
gaps (200-LOC rule violated by 8 files). Nothing blocks shipping but the
"pluggable" claim and dispatch-table doc both warrant immediate cleanup.
