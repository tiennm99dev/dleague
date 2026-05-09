# Phase 06 — Test coverage + local CI

## Context Links
- [Server review](reports/code-reviewer-server-260509-1331.md) — M14
- [Web review](reports/code-reviewer-web-260509-1331.md) — M15, M16, M17
- [Architecture review](reports/code-reviewer-architecture-260509-1331.md) — "Suggested additions: integration test for full match lifecycle"
- Depends on: Phase 01, 02, 05 (test against fixed code, not buggy code)

## Overview
- **Priority:** P2
- **Status:** pending
- **Description:** Close the test gap. Server has zero handler-level tests for the highest-risk Mongo-tx paths. Web has only `colors.test.ts`. Add handler tests, web WS-client + a11y tests, basic e2e for three play modes (solo, async, sync), and a CI workflow that runs lint+test on PR (no deploy).

## Key Insights
- Server: no tests for `match_handler.go`, `game_handler.go`, `sync_match_handler.go`, `leaderboard_handler.go` (server M14). These are the most security-sensitive Mongo-tx paths.
- Server has no fuzz tests for `proto.Unmarshal` paths (server M14).
- Server has no integration test driving two goroutines through `JoinAsChallengee` against a real `mongod --replSet` (server M14).
- Web has only `web/src/lib/colors.test.ts` (web M15).
- Web has no eslint/prettier (web M16).
- Web `tsconfig.json:16-17` has `noUnusedLocals/noUnusedParameters: false` (web M17).
- Existing CI: `.github/workflows/ci.yml` runs Go test + Mongo + Firebase emulator (arch strengths) — exists; add web pipeline.
- **Phase 05 specific tests:** state-filter audit (concurrent `JoinAsChallengee` with `state:"pending"` guard), leaderboard threshold guard (synthetic 6000-match day triggers `ErrLeaderboardTooLarge`), `parseDBName` fail-fast (malformed URI rejected at boot).

## Requirements
- Each handler file gets at least 1 happy-path + 2 error-path tests.
- Race-aware tests for the concurrency fixes from Phase 01.
- Web: WS-client tests (reconnect, pending rejection, token refresh chain), a11y smoke per route, basic e2e covering solo / async / sync paths.
- ESLint + Prettier on web with minimum config.
- CI workflow: lint + test on push/PR; no deploy steps.

## Related Code Files
**Modify**
- `web/package.json` (add lint/format scripts; add eslint+prettier deps)
- `web/tsconfig.json` (turn on `noUnusedLocals: true`; keep `noUnusedParameters: false`)
- `.github/workflows/ci.yml` (add web job; add lint step)

**Create — server tests**
- `server/internal/ws/match_handler_test.go` (challenge create/join/submit)
- `server/internal/ws/game_handler_test.go` (solo wordle session)
- `server/internal/ws/sync_match_handler_test.go` (queue-pair-resolve)
- `server/internal/ws/leaderboard_handler_test.go` (refresh + query)
- `server/internal/ws/auth_refresh_race_test.go` (Phase 01 H2 regression)
- `server/internal/ws/dispatch_fuzz_test.go` (`go test -fuzz` over envelope)
- `server/internal/store/matches_concurrency_test.go` (two-goroutine JoinAsChallengee against `mongo:7` testcontainer)

**Create — web tests**
- `web/src/lib/ws.test.ts` (mock WebSocket via `vitest`+`happy-dom`; reconnect backoff, pending rejection on close, token refresh chain, fresh-token on reconnect)
- `web/src/lib/auth-store.test.ts` (`idToken(force)` overload behaviour)
- `web/src/lib/components/board.test.ts` (renders, color classes by Color enum)
- `web/src/lib/components/keyboard.test.ts` (emits canonical Enter/Backspace; tabindex)
- `web/src/lib/components/results-screen.test.ts` (reason-prop variants)
- `web/e2e/solo.spec.ts` (Playwright; sign in anon → /play → submit guess)
- `web/e2e/async-pvp.spec.ts` (challenge create → second client joins → both submit → leaderboard)
- `web/e2e/sync-pvp.spec.ts` (two clients → quick-match → race → resolve)

**Create — config**
- `web/.eslintrc.cjs` (with `eslint-plugin-svelte` + `@typescript-eslint`)
- `web/.prettierrc` (with `prettier-plugin-svelte`)
- `web/playwright.config.ts` (basic; no remote browsers; chromium only)

## Implementation Steps

### Server handler tests
1. Stand up shared test fixtures: `server/internal/ws/testdeps_test.go` already has hub/conn helpers per existing tests; reuse pattern.
2. `match_handler_test.go`: cases `(a)` create challenge with valid token returns CHALLENGE_ACK; `(b)` join with wrong token → 404 envelope; `(c)` double-submit attempt → second returns ErrAttemptExists semantically; `(d)` submit with `time_ms` out of range → 422.
3. `game_handler_test.go`: `(a)` solo guess advances state; `(b)` invalid word → ErrNotInDictionary; `(c)` two tabs same UID — document current behaviour (per server H7 unresolved Q5).
4. `sync_match_handler_test.go`: `(a)` two conns QUEUE_JOIN → QUEUE_MATCHED on both; `(b)` queue eviction after 60s; `(c)` move during match → opponent gets MATCH_OPPONENT_PROGRESS; `(d)` win → MATCH_RESOLVED on both.
5. `leaderboard_handler_test.go`: `(a)` query returns sorted top-100; `(b)` query empty day → empty list.
6. `auth_refresh_race_test.go`: spawn 2 goroutines — one calling `handleAuthRefresh` repeatedly, another calling a function that reads `c.UserID()`. Run under `-race`. Should pass after Phase 01 step 2-4.
7. `dispatch_fuzz_test.go`: `go test -fuzz=FuzzDispatch` driving random bytes through envelope unmarshal + dispatch; assert no panic.
8. `matches_concurrency_test.go`: requires `mongo:7` replset. Use `MONGO_TEST_URI` env (already in CI). Two goroutines call `JoinAsChallengee` for same match concurrently → exactly one succeeds, the other returns `ErrAlreadyJoined`.

### Web tests
9. Add deps: `vitest`, `@testing-library/svelte`, `happy-dom`, `@playwright/test`. Add scripts: `test:unit`, `test:e2e`, `lint`, `format`.
10. `ws.test.ts`: mock global `WebSocket`. Cases: `(a)` reconnect after onclose; `(b)` pending promise rejected on close (Phase 01 step 13); `(c)` `idToken()` called per reconnect attempt (Phase 01 step 14); `(d)` token-refresh chain re-arms after reconnect.
11. Component tests: render via `render()` from `@testing-library/svelte`; assert DOM + ARIA + emitted events.
12. E2E: `playwright.config.ts` with `webServer: { command: 'npm run dev', port: 5173 }` and a separate command to start the Go server (or rely on CI compose). Use Firebase emulator. Skip in `npm test` by default; only run via `npm run test:e2e`.

### Lint + format
13. Create `web/.eslintrc.cjs`:
    ```js
    module.exports = {
      root: true,
      extends: [
        'eslint:recommended',
        'plugin:@typescript-eslint/recommended',
        'plugin:svelte/recommended'
      ],
      parser: '@typescript-eslint/parser',
      overrides: [{ files: ['*.svelte'], parser: 'svelte-eslint-parser', parserOptions: { parser: '@typescript-eslint/parser' } }],
      ignorePatterns: ['dist/', 'src/lib/pb/']
    };
    ```
14. Create `web/.prettierrc`:
    ```json
    { "semi": true, "singleQuote": true, "trailingComma": "none", "useTabs": true, "plugins": ["prettier-plugin-svelte"] }
    ```
15. `web/package.json` scripts: `"lint": "eslint . --ext .ts,.svelte"`, `"format": "prettier -w 'src/**/*.{ts,svelte}'"`.
16. `web/tsconfig.json:16` — `noUnusedLocals: true`. Fix any flagged dead code.

### CI workflow
17. `.github/workflows/ci.yml` — add a `web` job:
    - `node: 20`
    - `npm ci`
    - `npm run check` (existing)
    - `npm run lint`
    - `npm run test:unit`
18. Keep server job as-is. **Do NOT add deploy steps.**
19. Add a `proto` job that runs `make proto-gen` then `git diff --exit-code` to catch un-committed regen drift.

## Todo List
- [ ] Server handler tests (steps 1-5). **Phase 02 note:** Add coverage for `UIDLimiter` (race + TTL evict), `RedactUID` (determinism + hex format), `AttemptSubmit` 422 path, force-refresh cap (1/min).
- [ ] Auth-refresh race test (step 6)
- [ ] Dispatch fuzz (step 7)
- [ ] Mongo concurrency test (step 8)
- [ ] Web testing setup (vitest + playwright deps + scripts) (step 9). **Phase 03 note:** Add test for `results-screen.svelte` reason-prop mapping (exhausted→loss, opponent-left, self-disconnect).
- [ ] WS-client + auth-store + component unit tests (steps 10-11). **Phase 03 note:** `results-screen.test.ts` should cover all reason variants; `connection-status.test.ts` should verify Reconnect button disabled state while connecting.
- [ ] E2E happy paths (step 12). **Phase 03 note:** Smoke-test anonymous-warning banner on /play and reconnect affordance (disconnect → Reconnect button appears, click → reconnect).
- [ ] ESLint + Prettier (steps 13-15)
- [ ] tsconfig `noUnusedLocals` (step 16)
- [ ] CI web job + proto-drift job (steps 17-19)

## Success Criteria
- `go test ./... -race` passes on CI; coverage report shows `>50%` on `server/internal/ws/`.
- `cd web && npm run lint && npm run test:unit` passes on CI.
- `npm run test:e2e` passes locally against running server+emulator.
- CI run on a PR shows: server tests, web check+lint+unit, proto-drift; no deploy steps.
- Fuzz target catches no panics in 30s warm-up; ready for longer fuzz on demand.

## Risk Assessment
- **E2E flakiness:** Playwright tests depending on Firebase emulator + Mongo can flake. Mitigation: keep e2e out of default CI; document as `make e2e` for manual gate.
- **Mongo testcontainer in CI:** existing CI has Mongo service; reuse. Slow (~30s startup) — accept.
- **noUnusedLocals=true** may surface real dead code that wasn't intentional. Mitigation: fix as we go; if cascade is large, defer to Phase 07 cleanup.
- **Adding eslint may flag many existing files:** start with `--max-warnings=N` (current count) and reduce over time, OR fix all warnings up front.

## Security Considerations
- Fuzz testing surfaces parser panics that could be DoS vectors.
- Lint can detect log-of-credential patterns if a custom rule is added (deferred; Phase 02 logging changes are sufficient now).

## Next Steps
- After this phase, all Phase 01 fixes are pinned by tests.
- **Phase 07 refactors (steps 7-9: split ws.ts + play/+page.svelte) should land immediately after test suite passes.** Refactors touching hot paths are safer once behavior is pinned.
- Performance/load tests (200-conn synthetic) deferred to v2 per arch review's roadmap-additions list.
