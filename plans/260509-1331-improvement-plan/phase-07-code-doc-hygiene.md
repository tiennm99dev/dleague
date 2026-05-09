# Phase 07 — Code/doc hygiene

## Context Links
- [Architecture review](reports/code-reviewer-architecture-260509-1331.md) — M1, H4, M4, M5, M10, M11, M12, L1-L14
- [Server review](reports/code-reviewer-server-260509-1331.md) — M9, M10, M11, M12, M15, L2-L14
- [Web review](reports/code-reviewer-web-260509-1331.md) — M1, M2, M3, M8, M9, M10, M13, M14, L1-L28
- Depends on: Phase 04 (proto rename feeds dispatch-table doc rewrite)

## Overview
- **Priority:** P3
- **Status:** completed
- **Description:** Cleanup pass: 200-LOC violations in 8 Go + 3 Svelte files, stale `system-architecture.md` dispatch table, Go-version drift, closed-phase TODOs, `repomix-output.xml` lingering in worktree, daily-seed bias note, `code-standards.md` self-compliance audit, eslint-config rules around generated code. Group into themed batches; many small fixes.

## Key Insights
- `docs/system-architecture.md:118-121` lists message types that don't exist (`CREATE_MATCH`, `JOIN_MATCH`, `SUBMIT_ATTEMPT`, `MATCH_RESULT`) (arch H4).
- 8 Go files exceed 200-LOC cap from `code-standards.md:8`: `match_handler.go` 342, `conn.go` 312, `match_room_test.go` 299, `sync_match_handler.go` 279, `match_room.go` 268, `conn_test.go` 263, `leaderboards.go` 259, `matches.go` 227 (arch M1).
- 3 web files exceed: `play/+page.svelte` 345, `ws.ts` 364, `sync-game-scene.svelte` 233 (arch M1, web M1).
- `Dockerfile:10` Go 1.24 vs `go.mod:3` Go 1.26 (arch M2) — **out of scope per brief (deploy file)** but the same drift appears in `system-architecture.md:90` "Go 1.23" (arch L7) which IS in scope.
- `daily.go:58` modulo bias on seed → wordlist (arch M4).
- Closed-phase TODOs: `wordle.go:122` (Phase 10), `wordlist.go:1-7` (Phase 10), `store/games.go` (Phase 07 — covered in Phase 04) (arch L4-L6).
- `repomix-output.xml` 518 KB in worktree (arch H6) — gitignored but persistent.
- `web/src/lib/components/leaderboard-table.svelte` is unused; `leaderboard/+page.svelte:104-124` inlines a duplicate (web M10).
- `EventBus` payload typing is `unknown` (web M3).
- `OpponentPanel.svelte` + `sync-game-scene.svelte` use raw enum integers instead of `Color.GREEN` (web M8).
- `firebase.config.json` placeholder values; no env override (web H12).

## Requirements
- All in-scope docs match code reality.
- Files <200 LOC OR `code-standards.md` updated to carve out explicit exceptions (handlers/tests/generated) with rationale.
- No closed-phase TODOs in code.
- Stale 518KB artifact does not persist after `make clean`.
- DRY: no duplicated leaderboard table; no duplicated `formatTime`.
- Type safety: EventBus typed; opponent panel uses Color enum, not magic numbers.
- Firebase config supports per-env override.

## Related Code Files
**Modify**
- `docs/system-architecture.md` (dispatch table, Go version, status header, Atlas section already added in Phase 05)
- `docs/codebase-summary.md` (Game-interface description, Atlas note)
- `docs/code-standards.md` (carve-outs for handlers/tests/generated, OR keep strict and refactor)
- `Makefile` (add `clean` target to remove `repomix-output.xml`; add `seed-wordlists-prod` to `.PHONY`)
- `README.md` (drop "(planned)" from repo layout heading; pluggable claim already handled in Phase 04)
- `server/internal/ws/match_handler.go` (split into `match_handler.go` + `match_attempt_handler.go` + helpers)
- `server/internal/ws/conn.go` (split read/write loops into `conn_read.go`/`conn_write.go`)
- `server/internal/ws/sync_match_handler.go` (extract validate/displayName helpers)
- `server/internal/ws/match_room.go` (extract resolve helpers)
- `server/internal/store/leaderboards.go` (extract refresh helpers)
- `server/internal/store/matches.go` (split create/join/complete)
- `server/internal/game/wordle/wordle.go` (drop `Phase 10 can optimise` comment; add map-cache for dict — server M9)
- `server/internal/game/wordle/wordlist.go` (drop `TODO(phase-10)` block; log dropped count)
- `server/internal/game/wordle/daily.go` (comment on modulo bias acceptance)
- `server/cmd/seed-wordlists/main.go` (rename env var to `MONGO_URI` — server L6)
- `web/src/routes/play/+page.svelte` (extract `lib/play-controller.ts`)
- `web/src/lib/ws.ts` (split into `ws.ts` + `ws-reconnect.ts`)
- `web/src/lib/components/sync-game-scene.svelte` (extract a small controller module)
- `web/src/routes/leaderboard/+page.svelte` (use `leaderboard-table.svelte`; drop inline; share `formatTime`)
- `web/src/lib/components/leaderboard-table.svelte` (already exists; use it)
- `web/src/lib/event-bus.ts` (typed Events map)
- `web/src/lib/components/opponent-panel.svelte` (use `Color` enum)
- `web/src/lib/components/sync-game-scene.svelte` (use `Color` enum)
- `web/src/lib/firebase.ts` (read `import.meta.env.VITE_FIREBASE_*` with JSON fallback)
- `web/.env.example` (document VITE_FIREBASE_*)
- `web/src/lib/format-time.ts` (new — share)

**Delete**
- `repomix-output.xml` (and add to `make clean`)

## Implementation Steps

### Doc fact-fix
1. `docs/system-architecture.md:118-121` — replace dispatch table rows with actual message types (post Phase 04 rename): `CHALLENGE_CREATE / CHALLENGE_JOIN / ATTEMPT_SUBMIT / WORDLE_MOVE / WORDLE_STATE / QUEUE_JOIN / QUEUE_MATCHED / MATCH_OPPONENT_PROGRESS / MATCH_RESOLVED / MATCH_FORFEIT / MATCH_REJOIN / MATCH_REJOIN_ACK / AUTH_REFRESH`. Source-of-truth: `proto/dleague/v1/envelope.proto`.
2. `docs/system-architecture.md:90` — change "Go 1.23" → "Go 1.26".
3. `docs/system-architecture.md:5` — drop "skeleton — diagrams + ERD landed by Phase 10" status line; replace with "current".
4. `docs/codebase-summary.md:61-63` — covered in Phase 04 step 3.
5. `README.md:56` — drop "(planned)" from "Repo layout (planned)" heading.

### `code-standards.md` self-compliance
6. **Decision: relax 200-LOC to a guideline with carve-outs** (per arch M1 unresolved Q3). Update `docs/code-standards.md:8`:
   ```
   - Soft cap 200 LOC per file. Carve-outs (no refactor required):
     - WS handler files (`*_handler.go`) — switch-heavy dispatch makes splits artificial.
     - Test files — fixture setup + multiple cases naturally exceed.
     - Generated code (`shared/pb/`, `web/src/lib/pb/`) — never edited.
     Files outside carve-outs SHOULD split when crossing 250 LOC.
   ```
7. With the relaxed rule, only 4 files clearly need split: `web/src/routes/play/+page.svelte` (345), `web/src/lib/ws.ts` (364), `server/internal/store/leaderboards.go` (259, borderline — defer), `web/src/lib/components/sync-game-scene.svelte` (233 — defer).
8. Split `web/src/routes/play/+page.svelte`: extract a `web/src/lib/play-controller.ts` module owning `applyServerState`, `submitAttempt`, `createChallenge`, `submitGuess`. Component shrinks to UI binding.
9. Split `web/src/lib/ws.ts`: extract `web/src/lib/ws-reconnect.ts` with reconnect/scheduleReconnect/scheduleTokenRefresh logic; `ws.ts` keeps connect/dispatch/handlers.

### Cleanup TODOs + comments + dead code
10. `server/internal/game/wordle/wordle.go:122` — drop "Phase 10 can optimise" sentence. Implement the map-cache (per server M9) only if cheap; otherwise drop the comment without the impl.
11. `server/internal/game/wordle/wordlist.go:1-7` — drop the `TODO(phase-10)` block; backlog item already in `development-roadmap.md`.
12. `server/internal/game/wordle/daily.go:58` — add comment: "Modulo bias acceptable: with ~772 answers and 2^63 seed range, bias is ~1 in 2^53 — undetectable."
13. `server/internal/game/wordle/wordlist.go:parseWordList` — log dropped malformed lines count: `log.Printf("wordle: parseWordList dropped %d malformed lines from %s", dropped, path)`.
14. `server/cmd/seed-wordlists/main.go:24` — rename `DLEAGUE_MONGO_URI` → `MONGO_URI` to match `config.go:91` (server L6). Update help text + README.
15. `repomix-output.xml` — delete now. Add `clean` target to `Makefile`: `clean: ; rm -f repomix-output.xml`.
16. `Makefile:9` — add `seed-wordlists-prod` to `.PHONY` (arch L3) — but this is deploy-adjacent; verify it's not a deploy target before editing. If deploy: skip per scope.

### Web DRY + types
17. Create `web/src/lib/format-time.ts` exporting `formatTime(ms: number): string`. Replace inline copies in `leaderboard/+page.svelte:41` and `leaderboard-table.svelte:11`.
18. `web/src/routes/leaderboard/+page.svelte:104-124` — replace inline table markup with `<LeaderboardTable entries={...} />`. Delete the inline implementation.
19. `web/src/lib/event-bus.ts` — typed Events map per web M3:
    ```ts
    type Events = { 'title:start': []; 'wordle:flip-row': [FlipRowPayload] };
    class EventBus {
      on<K extends keyof Events>(e: K, h: (...args: Events[K]) => void): void;
      emit<K extends keyof Events>(e: K, ...args: Events[K]): void;
    }
    ```
20. `web/src/lib/components/opponent-panel.svelte:22-33` and `web/src/lib/components/sync-game-scene.svelte:56-62` — replace `case 3:` etc. with `case Color.GREEN:` (import `Color` enum).

### Firebase env override
21. `web/src/lib/firebase.ts` — read each config key as `import.meta.env.VITE_FIREBASE_API_KEY ?? jsonConfig.apiKey` etc. JSON file remains as fallback for local dev.
22. `web/.env.example` — document `VITE_FIREBASE_API_KEY`, `VITE_FIREBASE_AUTH_DOMAIN`, `VITE_FIREBASE_PROJECT_ID` etc. with note "leave unset to use firebase.config.json".

### Smaller nits (batch into 1 commit)
23. `web/src/lib/ws.ts:113` `onerror` — `console.warn('ws onerror', e)` (web L1).
24. `web/src/lib/ws.ts:160` reject — include server's error payload (web L2).
25. `web/src/lib/ws.ts:208` `onMessage` — `console.warn` if overwriting (web L3).
26. `web/src/lib/components/sign-in.svelte:18` — friendly errors covered in Phase 03.
27. `server/internal/ws/match_handler.go:148-150` — extract constants `minTimeMs`/`maxTimeMs` (arch L12).
28. `server/internal/ws/sync_match_handler.go:267` — change `os.Exit(1)` → covered in Phase 01 step 5; this reference removed.
29. `server/internal/store/leaderboards.go:51,253` `mustParseDate` — return error rather than panic (server M10).

## Todo List

### Doc fact-fix batch
- [x] Dispatch table + Go version + status line (steps 1-3)
- [x] README "(planned)" drop (step 5)
- [x] code-standards 200-LOC carve-out (step 6)

### Refactor batch
- [x] Extract `play-controller.ts` (step 8) — deferred to Phase 06
- [x] Extract `ws-reconnect.ts` (step 9) — deferred to Phase 06

### Code cleanup batch
- [x] Drop closed-phase TODOs + stale comments (steps 10-12)
- [x] Wordlist drop count logging (step 13)
- [x] seed-wordlists env rename (step 14)
- [x] Delete `repomix-output.xml` + Makefile clean (step 15)
- [x] Wordle dict map-cache (step 10 optional) — skipped

### Web DRY + types batch
- [x] `format-time.ts` shared (step 17)
- [x] Use `LeaderboardTable` component (step 18)
- [x] Typed EventBus (step 19)
- [x] Color enum usage (step 20)

### Firebase override batch
- [x] env override + .env.example (steps 21-22)

### Nits batch
- [x] WS error/warn logging (steps 23-25)
- [x] Time-ms constants (step 27)
- [x] mustParseDate → error (step 29)

## Success Criteria
- `grep -nE "TODO\(phase-(0[7-9]|10)\)" .` returns 0 hits.
- `wc -l server/internal/ws/match_handler.go web/src/routes/play/+page.svelte web/src/lib/ws.ts` show <250 (post-split for unboxed files; carve-outs documented).
- `grep -rn "CREATE_MATCH\|JOIN_MATCH\|SUBMIT_ATTEMPT\|MATCH_RESULT" docs/` returns 0 hits in dispatch-table contexts.
- `make clean && ls repomix-output.xml` returns "No such file".
- `npm run check` passes after EventBus retype.
- `web/src/lib/firebase.ts` reads VITE_FIREBASE_* env vars; build with `VITE_FIREBASE_PROJECT_ID=test123 npm run build` produces config with that ID.

## Risk Assessment
- **Refactor regressions:** splitting `play/+page.svelte` and `ws.ts` is mechanical but touches hot paths. Mitigation: Phase 06 tests pin behaviour; do refactor after tests.
- **Code-standards relaxation may invite drift:** soft cap can creep. Mitigation: explicit "split at 250" trigger phrased clearly.
- **EventBus retype may cascade:** all `eventBus.emit` callers must match the typed map. Mitigation: TypeScript will surface every mismatch at `npm run check` time.
- **Firebase env override:** existing `firebase.config.json` workflow keeps working since env is a fallback layer. Low risk.

## Security Considerations
- Wordlist drop-count logging surfaces silent data corruption (e.g., truncated download).
- `mustParseDate` error path prevents scheduler panic on user-supplied date — defense against future input.

## Completion Notes (2026-05-09)

**Status:** COMPLETED. 22 in-scope steps landed across doc fact-fix, code-standards carve-out, dead TODO cleanup, format-time DRY, typed EventBus, Color enum fixes, firebase env override, ws logging nits, and build-artifact cleanup.

**Reports:**
- `reports/code-reviewer-phase-07-diff-260509-1502.md` — final implementation diff + notes
- `reports/tester-phase-07-260509-1502.md` — tests + `npm run check` green

**Deferred (documented):**
- **Steps 7-9 (refactor: split play/+page.svelte and ws.ts):** Implementer correctly noted these should land **after Phase 06 tests pin behavior**, not before. Refactors touching hot paths are safer post-test-suite. Recommend: Phase 06 → then split ws.ts/play routes as first commits of next session.

**Review fix-ups (2):**
- I-1: Corrected 5 stale GAME_MOVE/STATE enum references in `system-architecture.md` prose (outside dispatch table). Updated to actual message types.
- I-2: Extracted `minTimeMs`/`maxTimeMs` constants in `match_handler.go:148-150`; error message now wired to constants instead of hardcoded `[500, 86400000]`.

## Next Steps
- After this phase, all three reviews' findings are addressed or explicitly deferred with rationale.
- **Phase 06 (test infrastructure) pending.** When implemented, follow immediately with refactors from Phase 07 steps 7-9 as part of test-driven refactor cycle.
- v2 backlog (out of this plan): structured logging via `slog`, leaderboard aggregation pipeline, real 2315-word wordlist, integration test for full match lifecycle.
