# Phase 07 — code/doc hygiene diff review

## Verdict
**APPROVE_WITH_FIXES** — all 22 in-scope steps land. Two minor doc-fact misses (stale `GAME_MOVE`/`GAME_STATE` references outside the dispatch table that the spec didn't enumerate but Phase 04 made stale) and one borderline carve-out gap. No correctness regressions; build/vet/race/svelte-check/web-tests all green.

## Build/test sanity
- `go build ./...` — clean
- `go vet ./...` — clean
- `go test -race ./...` — all packages pass
- `web npm run build` — 17.65s, succeeds
- `web npx svelte-check` — 401 files, 0 errors, 0 warnings
- `web npm test` — 9/9 pass
- `repomix-output.xml` — gone
- `grep -rn "TODO(phase-"` over server/, shared/, proto/, web/src/, docs/, Makefile, README.md — 0 hits

## Spec compliance

| # | Step | Status | Evidence |
|---|------|--------|----------|
| 1 | Dispatch table message types | ✅ | `docs/system-architecture.md:113-128` enumerated correctly against `proto/dleague/v1/envelope.proto` |
| 2 | Go 1.23 → 1.26 | ✅ | `docs/system-architecture.md:90` |
| 3 | "skeleton" → "current" | ✅ | `docs/system-architecture.md:3` |
| 4 | codebase-summary Game-interface | n/a | per spec: covered in Phase 04 |
| 5 | README "(planned)" drop | ✅ | `README.md:56` reads "Repo layout" |
| 6 | code-standards 200-LOC carve-out | ✅ (with caveat) | `docs/code-standards.md:7-13`; see I-3 |
| 7-9 | Refactor extracts | DEFERRED | per scope brief; tracked as open follow-ups |
| 10 | Drop "Phase 10 can optimise" comment | ✅ | `wordle.go:122-123` (now O(n) lin-search comment, no phase ref); no dangling block |
| 11 | Drop TODO(phase-10) block | ✅ | `wordlist.go:1-12` clean |
| 12 | Modulo-bias comment | ✅ | `daily.go:58` |
| 13 | Wordlist drop count log | ✅ | `wordlist.go:79-81`; gated `if dropped > 0` — not noisy |
| 14 | DLEAGUE_MONGO_URI → MONGO_URI | ✅ | `seed-wordlists/main.go:7,24` |
| 15 | Delete repomix + Makefile clean | ✅ | file gone; `Makefile:97-99` `rm -f` (idempotent) |
| 16 | seed-wordlists-prod .PHONY | ✅ | `Makefile:7-9` |
| 17 | Shared format-time | ✅ | `web/src/lib/format-time.ts` (new); used by `leaderboard-table.svelte:6` (only consumer post-step-18) |
| 18 | Use LeaderboardTable | ✅ | `leaderboard/+page.svelte:14,85`; inline removed |
| 19 | Typed EventBus | ✅ | `event-bus.ts` Events map covers ALL emit sites (`title:start`, `wordle:flip-row`); see I-4 |
| 20 | Color enum usage | ✅ | `opponent-panel.svelte:24-29`, `sync-game-scene.svelte:60-62` |
| 21 | Firebase env override | ✅ | `firebase.ts:23-32` `env.VITE_FIREBASE_*  ?? jsonConfig.x` |
| 22 | .env.example | ✅ | 6 entries documented |
| 23 | ws onerror warn | ✅ | `ws.ts:119-122` |
| 24 | ERROR envelope decode + reject | ✅ | `ws.ts:193-200` decodes ErrorSchema |
| 25 | onMessage overwrite warn | ✅ | `ws.ts:253-255`; see I-5 |
| 26 | sign-in friendly errors | n/a | covered in Phase 03 |
| 27 | minTimeMs/maxTimeMs constants | ✅ | `match_handler.go:16-19,162` |
| 28 | os.Exit(1) — Phase 01 | n/a | already done |
| 29 | mustParseDate → parseDate | ✅ | `leaderboards.go:59-63,277-283`; one callsite (implementer note correct); fully removed (grep `mustParseDate` returns 0) |

## Issues

### High
None.

### Medium

- **I-1 — stale `GAME_MOVE`/`GAME_STATE` in `docs/system-architecture.md` outside dispatch table.**
  `docs/system-architecture.md:254` (Game flow ASCII diagram), `:267` (server reply arrow), `:304-305` ("Wire format" bullets), `:324` (sync-PvP flow). All reference `GAME_MOVE`/`GAME_STATE` post-Phase-04 rename to `WORDLE_MOVE`/`WORDLE_STATE`.
  *Why it matters:* the doc-fact-fix scope was explicitly to align dispatch-table with `envelope.proto`. The dispatch table is now correct, but the surrounding prose still cites the old names — same class of bug as the original H4. Implementer applied the fix narrowly to lines 118-121 per spec wording; spec missed these.
  **Fix:** s/GAME_MOVE/WORDLE_MOVE/g and s/GAME_STATE/WORDLE_STATE/g in the file (5 hits). Also update the bracketed enum value comment on `:304` ("MESSAGE_TYPE_GAME_MOVE` (6)") since that integer value 6 is now `WORDLE_MOVE`.

- **I-2 — `match_handler.go:163` error message contains hardcoded numbers, not constants.**
  `errorEnvelope(... "time_ms out of range [500, 86400000]" ...)`.
  *Why it matters:* Step 27 extracted `minTimeMs`/`maxTimeMs`. The user-facing message still hardcodes the same numbers — if someone tunes the constants the message drifts.
  **Fix:** `fmt.Sprintf("time_ms out of range [%d, %d]", minTimeMs, maxTimeMs)` or accept that the message is a contract.

### Low

- **I-3 — code-standards carve-outs leave gaps.** `docs/code-standards.md:8-13` covers:
  - `*_handler.go` ✓ — match_handler.go (342), sync_match_handler.go (279)
  - test files ✓ — match_room_test.go (299), conn_test.go (263)
  - generated code ✓
  - "stateful infrastructure modules (`web/src/lib/ws.ts`, `server/internal/ws/conn.go`)" ✓ — but this is enumerated as a closed list (named files), not a category. Implication: any other infra file would need explicit naming.

  Files that still cross 250 LOC and are NOT covered by any carve-out:
  - `server/internal/store/leaderboards.go` (283 LOC) — store module, not handler/test/infra
  - `server/internal/ws/match_room.go` (270 LOC) — sync match room (not a `*_handler.go`, not infra by name)
  - `server/internal/store/matches.go` (234 LOC) — under 250 trigger but borderline
  - `web/src/routes/play/+page.svelte` (329 LOC) — route page, not under any carve-out
  - `web/src/lib/components/sync-game-scene.svelte` (234 LOC) — under trigger but borderline

  Carve-out gaming risk is low (named-file approach prevents drift) but the rule as written would force `match_room.go` and `leaderboards.go` to split per the >250 trigger — they're explicitly deferred per phase scope, so the standard contradicts itself today. **Fix options:** (a) extend infra carve-out to a category ("stateful sync state — match rooms, queues, ws conns"); (b) note in `code-standards.md` that route page files (`+page.svelte`) follow the same handler exemption logic.

- **I-4 — EventBus emit() casts through `Function` then `unknown[]`.** `event-bus.ts:23,41`. Implementer disabled `@typescript-eslint/no-unsafe-function-type` on the listeners map. Public API stays type-safe (the generic constraint on `on`/`emit` does the work). Acceptable — comment on line 21-22 explains the limitation. Minor: the `unknown[]` cast in `emit` could be tightened by storing `Handler<keyof Events>` arrays in a discriminated union; deferred (YAGNI for 2 events).

- **I-5 — `onMessage` overwrite warn fires legitimately.** `ws.ts:253-255` warns whenever a handler is replaced. `sync-game-scene.svelte:97-105` registers a `WORDLE_STATE` handler on every mount; if a user navigates `/sync` → `/play` → `/sync` (or hot-reloads in dev), this will warn on each remount. Not a defect (warn is right thing on overlap) but expect dev-console noise. **Suggestion:** consider a separate `replaceHandler()` API for the legitimate case, or downgrade to `console.debug`. Defer.

- **I-6 — `firebase.ts:23` reads `import.meta.env` once at module top.** Vite inlines `import.meta.env.VITE_*` at build time, so this is correct: build-time substitution as the spec intended. The behavior is per-build, not per-runtime — clearly documented in firebase.ts:5 comment ("CI/prod > local dev default"). No action.

- **I-7 — `ws.ts onerror` logs `e` directly.** `ws.ts:120` `console.warn('ws onerror', e)`. `e` is `Event` (browser `WebSocket.onerror`), not Error. `console.warn` handles non-Error fine; safe. No action.

- **I-8 — ERROR envelope decode resilience.** `ws.ts:193-200` catches decode failure and falls back to a generic message. Handles missing/malformed proto gracefully. No action.

## Strengths

- `parseWordList` retained as a public wrapper around `parseWordListNamed` for backward compat — all 7 callers in `wordlist.go` and `wordlist_test.go` keep their original signature.
- Drop-count logging gated `if dropped > 0` — silent on clean files (matches phase intent of surfacing data corruption only).
- `parseDate` returns error and the single scheduler call site (`leaderboards.go:59-63`) logs WARN + skips — graceful degradation, no panic propagation.
- `Makefile:99` `rm -f` is idempotent (no error if file gone).
- Color enum imports switched from `type Color` to value `Color` in `opponent-panel.svelte:2` — required for runtime switch comparison.
- `.env.example` placeholders empty (not committed-secret values) — safe template.
- Dispatch table includes `ERROR` as server→client only with explicit "no handler" annotation — anti-confusion.

## Open follow-ups

- **Step 7-9 deferred refactors:** `web/src/routes/play/+page.svelte` (329) and `web/src/lib/ws.ts` (411) per phase scope brief — pending Phase 06 tests as risk-mitigation. `server/internal/store/leaderboards.go` (283) still over the 250 trigger.
- **I-1:** sweep remaining stale `GAME_MOVE`/`GAME_STATE` references across `docs/system-architecture.md` (5 hits).
- **I-2:** parameterize the time-bounds error message with the constants.
- **I-3:** clarify code-standards carve-out language: `match_room.go`/`leaderboards.go`/`+page.svelte` either need explicit listing or a category-based carve-out.
- v2 backlog (out of plan): structured logging via slog; aggregation pipeline for leaderboards; full 2315-word wordlist; integration test for full match lifecycle.

## Unresolved questions

- Does the team want `LeaderboardTable.empty` paragraph rendered when `rankings.length === 0` to mirror the previous inline behavior? Current `leaderboard/+page.svelte:80-86` only renders the table when *not loading and* never empty — but `LeaderboardTable` has its own `.empty` branch. Behavior preserved correctly post-refactor; just confirm.
- For the EventBus, is the dispatch ordering guaranteed (Set→Array switch)? `event-bus.ts` uses `Function[]` (Array) — preserves registration order. Previous map probably did too; regression-free assumed.

**Status:** DONE_WITH_CONCERNS
**Summary:** Phase 07 ships cleanly across all 22 in-scope steps; build/test/lint green. Two doc-prose stale-name hits (system-architecture.md GAME_MOVE/GAME_STATE outside dispatch table) and a code-standards carve-out gap for match_room.go/leaderboards.go/+page.svelte are non-blocking but warrant a follow-up commit.
**Concerns:** I-1 (stale message-type names in arch doc prose); I-3 (carve-out covers named files but not categories — leaves match_room.go/leaderboards.go/route pages in limbo).
