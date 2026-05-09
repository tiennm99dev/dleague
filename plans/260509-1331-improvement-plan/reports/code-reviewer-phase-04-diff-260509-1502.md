# Phase 04 — Pluggability decision: diff review

## Verdict

**APPROVE_WITH_FIXES** — Mechanical work clean; build/check/race all green; proto rename + dead-code drop verified end-to-end. Two doc misses violate the phase's own success criterion (`grep -rni "pluggable" README.md docs/` → only v2 contexts). One TODO marker references a closed phase. Fixes are line-edits, not structural.

## Spec compliance

| # | Step | Status | Note |
|---|------|--------|------|
| 1 | README.md:12 bullet replaced | DONE | bullet now: "Wordle at launch — multi-game support is on the long-term roadmap, not in this release" |
| 2 | docs/project-overview-pdr.md pluggability sentence | DONE | line 17 reworded to "shared/game package contains a reserved (currently inactive) scaffold"; decisions table line 69 also updated |
| 3 | docs/codebase-summary.md interface signature + scaffold prefix | DONE | line 62 "Reserved scaffold (v2)"; line 63 lists actual `Init/Validate/Apply/IsTerminal/Result` |
| 4 | docs/development-roadmap.md pluggable under v2/exploratory | DONE | line 76-77 v2 backlog; status table line 16 has Phase-04 row |
| 5 | proto enum rename + values 6/7 preserved + comment | DONE | envelope.proto:15-17 — comment + WORDLE_MOVE=6, WORDLE_STATE=7 |
| 6 | make proto-gen ran | DONE | shared/pb/.../envelope.pb.go:35-36 + envelope_pb.ts:187-192 regenerated; descriptor bytes line 473-474 match |
| 7 | Go callsite renames | DONE | hub.go:112, match_room.go:98, game_handler.go:61/63/124, match_room_test.go:67/69 |
| 8 | TS callsite renames | DONE | play/+page.svelte:180, sync-game-scene.svelte:97/151 |
| 9 | go build + svelte-check green | DONE | go test -race ok across 6 pkgs; svelte-check 400/0/0/0 |
| 10 | Delete games.go + drop `_ = NewGameRepo` | DONE | file gone (`ls` confirms); main.go:95 region clean (no orphan ref); `grep NewGameRepo\|GameRepo` 0 hits |
| 11 | Drop KeyEnter/KeyBackspace | DONE | `grep KeyEnter\|KeyBackspace --include='*.go'` 0 hits |
| 12 | Scaffold-note comments on shared/game/* | DONE | game.go:1-5 + registry.go:1-2 — both link to phase-04 plan path |

Success-criterion check (spec line 88-92):
- `grep -rni "pluggable" README.md docs/` → **3 live present-tense hits remain** (see Issues #1, #2). Status-table rows in README.md:94 + roadmap.md:38 are historical phase names — fine.
- `go build ./...` green; `cd web && npm run check` green — confirmed.
- `grep MESSAGE_TYPE_GAME_MOVE|MessageType_GAME_MOVE|MessageType.GAME_MOVE` → 0 hits — confirmed.
- `grep TODO(phase-` → **1 hit** in wordlist.go (Issue #3).

## Issues

### #1 [HIGH] docs/code-standards.md:76 — present-tense pluggable claim
File: `docs/code-standards.md:73-78`
Section "Game Architecture / Game Interface (Phase 2)" reads:
- `**Define:** shared/game/Game interface with pluggable -dle types (Wordle, music, geography, etc.)`
- `**Registry:** Factory pattern in shared/game/registry.go — register games by ID, lookup at match start`

Both statements describe the design as currently active. Violates phase decision (scaffold-only) and spec line 88 ("only roadmap/v2 contexts"). Phase 04 step 4 is "drop the claim from docs". Phase plan's "Modify" list at line 38 names codebase-summary but not code-standards — implementer may have read the list literally. Spec is the contract though; this is a miss.

**Fix:** rewrite the Game Architecture section to mark scaffold-only, e.g.:
```
### Game Interface (reserved scaffold)
- Wordle at launch is constructed directly (`wordle.NewWordle`) — not via the interface.
- `shared/game/Game` interface + `shared/game/registry.go` are reserved scaffolds for future multi-game support; no factories registered at runtime.
- See plans/260509-1331-improvement-plan/phase-04-pluggability-decision.md.
```

### #2 [MEDIUM] docs/system-architecture.md:52 — present-tense pluggable claim in client tree
File: `docs/system-architecture.md:51-52`
Tree comment reads `game.ts — pluggable Game<S,M> interface + State/Move/Result types`. Phase 04 spec line 40 says system-architecture.md should "drop Game interface diagram if any references it". This is one such ref. Implementer's note ("no Game-interface diagram refs — no change needed") is wrong on this line.

**Fix (one-line):**
```
│   ├── game.ts                 — reserved game interface scaffold (v2; not used by current Wordle path)
```

### #3 [LOW] server/internal/game/wordle/wordlist.go:13 — TODO(phase-10) references closed phase
File: `server/internal/game/wordle/wordlist.go:13`
```
// TODO(phase-10): Replace placeholder word lists with the full public-domain
```
Phase 10 ("Deploy + polish") completed per README.md:97. The work itself is legitimately deferred (roadmap.md:50 lists it under v2 High-priority). But the marker pattern violates spec success criterion line 91 ("TODO(phase-XX) returns 0 hits") and is misleading — a future grep treats it as in-flight Phase-10 work.

**Fix:** rename `TODO(phase-10)` → `TODO(v2)` (matches the roadmap's v2 Backlog section).

### #4 [INFO] web/src/lib/game/game.ts — entire file is a parallel client-side pluggable scaffold not addressed
File: `web/src/lib/game/game.ts`
The TS mirror of shared/game/game.go exists and its top comment line 1 reads `Pluggable game interface — TypeScript mirror of shared/game/game.go`. Phase 04 spec only enumerates Go-side scaffolds (steps 11-12). This file is consistent with the v2-scaffold framing but lacks the same scaffold-note comment + plan link. Not blocking — out of literal scope of the phase. Worth a follow-up sweep for symmetry.

### #5 [INFO] buf breaking check not run
Spec line 95 marks `buf breaking --against '.git#branch=main'` as risk-mitigation. Implementer summary notes it wasn't run. Per architecture review there are no live external clients, so the rename is safe for this release. Note for record only.

## Strengths

- Proto rename is fully consistent end-to-end: descriptor bytes, Go const, TS enum, all callsites including a comment in game_handler.go:61/63 — no half-renamed call sites.
- Numeric values 6/7 preserved → wire-format compatible with any in-flight client builds.
- Scaffold-note comments on game.go:1-5 + registry.go:1-2 cite the exact plan path; future archaeology has a breadcrumb.
- `match_room_test.go` was modified to compile against the renamed enum — implementer flagged this as out-of-list and made the right call (build correctness > strict file-list adherence).
- Test suite still green under `-race` across all 6 server packages with tests; no leaked goroutine warnings.
- `_ = store.NewGameRepo(db)` cleanly removed; `store` package import still valid (other repos still used).
- Hints field stays dropped (Phase 05 invariant): `grep -rn 'hints' shared/pb` shows only WordleState's `repeated string hints` — that's the legitimate game state field, not the dropped move hints.

## Open follow-ups

1. (Q) Should `web/src/lib/game/game.ts` get the same scaffold-note treatment as Go side, or is the client mirror considered live (used by Wordle preview UI)? Skim of `web/src/lib/game/wordle/wordle.ts` likely reveals whether `WordleGame` actually `implements Game<S,M>` — if not, mirror is also dead and deserves the same demotion.
2. (Q) Phase 04 task in `docs/development-roadmap.md:16` is listed `pending` but this diff completes it — should the row flip to `completed (2026-05-09)` as part of this commit, matching the cadence on Phase 01/02/03/05?
3. (Q) `docs/project-changelog.md:112` ("Phase 07 — Game core pluggable + server-authoritative Wordle") — historical commit-message reproduction, leave alone? Or footnote that the pluggability layer was demoted to scaffold in Phase 04 hardening?

---

**Status:** DONE_WITH_CONCERNS
**Summary:** Mechanical proto + dead-code work is correct end-to-end (build green, race-clean, 0 stale GAME_MOVE refs). Two doc files (code-standards.md:76, system-architecture.md:52) still carry present-tense pluggable claims; one TODO marker references closed phase-10. All three are line-edits.
**Concerns/Blockers:** Issues #1-#3 should land before commit so the phase-04 success criterion grep is actually 0/0.
