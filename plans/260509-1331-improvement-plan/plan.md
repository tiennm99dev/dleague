---
title: "Dleague post-MVP hardening"
description: "Fix correctness/security/UX bugs and gaps surfaced by 3 code reviews; deploy out of scope"
status: pending
priority: P1
effort: ~6-8d
branch: main
tags: [hardening, bugfix, refactor, tests, docs]
created: 2026-05-09
---

## Goal

All 10 build phases shipped; three reviews (server / web / architecture) surfaced ~70 issues
ranging from data-corruption races to doc/code drift. This plan groups them by **theme + risk**
into 7 phases focused on correctness, security, UX repair, pluggability decision, persistence
hygiene, test infra, and code/doc cleanup. Goal: ship-ready post-MVP, no new game features.

## Out of Scope

- **Deployment:** Fly.io, Dockerfile, docker-compose, deploy scripts, prod secrets, rollout.
- **New game types:** keep wordle-only; Phase 04 only decides the contract or removes the claim.
- **Major perf rewrites:** leaderboard aggregation pipeline noted but deferred (M2 doc-only).

## Phases

| #  | Name                          | Priority | Status  | Description                                                          |
|----|-------------------------------|----------|---------|----------------------------------------------------------------------|
| 01 | Critical correctness          | P1       | completed | High-severity races/DoS/duplicate writes — must-fix before any trust |
| 02 | Security & abuse hardening    | P1       | completed | Token/UID logging, WS origin, rate limits, anti-cheat, token refresh |
| 03 | UX correctness                | P2       | completed | Sync Enter key, rejoin nav, anon warning, a11y, results edges        |
| 04 | Pluggability decision         | P2       | pending | Wire wordle through `shared/game.Game` OR drop the claim from docs   |
| 05 | Persistence & data integrity  | P1       | pending | Unique indexes, state filters, idempotency, Atlas docs, Hints field  |
| 06 | Test coverage + local CI      | P2       | pending | Handler tests, web WS/a11y/e2e tests, CI lint+test workflow          |
| 07 | Code/doc hygiene              | P3       | pending | 200-LOC violations, stale docs, drift, repomix, eslint/prettier      |

## Cross-phase dependencies

- **Phase 06 (tests) depends on Phase 01 + 02 + 05 landing.** Writing tests against buggy code
  bakes the bugs in. Land critical fixes first; tests pin the fixed behaviour.
- **Phase 04 (pluggability) is independent.** Recommended cheap path: shrink to a 1-task doc edit
  (delete "pluggable" claim; keep `shared/game.Game` interface frozen as future scaffold). If a
  second game is on the near roadmap, escalate to full rewire — but per `development-roadmap.md`
  it's "lower priority / exploratory", so doc-edit is the YAGNI/KISS choice. **Default: doc-edit.**
- **Phase 07 (hygiene) depends on Phase 04 outcome** for `system-architecture.md` dispatch table
  rewrite (which message types survive the rename).
- **Phase 03 H6 depends on Phase 01 H8 (web rejoin payload)** — fix payload threading first, then
  UX gating.

## Reports

- [Server review](reports/code-reviewer-server-260509-1331.md)
- [Web review](reports/code-reviewer-web-260509-1331.md)
- [Architecture review](reports/code-reviewer-architecture-260509-1331.md)

## Phase files

- [phase-01-critical-correctness.md](phase-01-critical-correctness.md)
- [phase-02-security-hardening.md](phase-02-security-hardening.md)
- [phase-03-ux-correctness.md](phase-03-ux-correctness.md)
- [phase-04-pluggability-decision.md](phase-04-pluggability-decision.md)
- [phase-05-persistence-integrity.md](phase-05-persistence-integrity.md)
- [phase-06-test-coverage.md](phase-06-test-coverage.md)
- [phase-07-code-doc-hygiene.md](phase-07-code-doc-hygiene.md)
