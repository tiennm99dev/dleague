# Phase 04 — Pluggability decision (doc-edit path)

## Context Links
- [Architecture review](reports/code-reviewer-architecture-260509-1331.md) — H1, H2, H3, L6, L8

## Overview
- **Priority:** P2
- **Status:** pending
- **Description:** `shared/game.Game` interface + `Registry` are dead code; PDR/README/roadmap promise pluggable game types but no code uses the abstraction. **Default (recommended) path: drop the claim from docs, freeze the interface as future scaffold.** Building a 2nd game is out of scope per the brief. This phase is intentionally minimal — one task batch of doc edits + one task batch of dead-code/TODO cleanup. If the team later commits to a 2nd game, escalate to a separate plan that wires wordle through the interface and adds proto `oneof Payload` or `game_id` field.

## Key Insights
- `shared/game/game.go:47-67` defines `Game` interface; `shared/game/registry.go` exposes `Register/New/IDs`. **Zero call sites** in `server/` or `web/` (arch H1).
- `wordle.Wordle` doesn't satisfy `Game`: signature `Validate(guess string, dict []string)` ≠ interface `Validate(move Move)` (arch H1).
- `server/cmd/api/main.go:95` does `_ = store.NewGameRepo(db)` — instantiated and discarded (arch H1).
- `server/internal/store/games.go:39-41` has `TODO(phase-07)` markers despite Phase 07 closed (arch L6).
- `proto/dleague/v1/envelope.proto:18-23` `MESSAGE_TYPE_GAME_MOVE/STATE` are wordle-shaped (carry `WordleMove`/`WordleState`) — locks values 6/7 to wordle semantics (arch H3).
- `docs/codebase-summary.md:61-63` describes interface signature that doesn't match actual code (arch L8).

## Requirements
- README + PDR + roadmap stop claiming "pluggable game types" in present tense.
- Dead code (`GameRepo`, the `_ =` discard, unused `KeyEnter`/`KeyBackspace` constants in `shared/game/game.go:38-40`) removed OR kept-with-explicit-rationale.
- `shared/game/game.go` + `registry.go` either deleted OR kept as scaffold with a top-of-file comment "Reserved scaffold for v2 multi-game; not currently used. See phase-04 decision."
- Proto `MESSAGE_TYPE_GAME_MOVE/STATE` either renamed to `WORDLE_MOVE/STATE` OR documented in proto comments as wordle-locked.
- No `TODO(phase-XX)` references to closed phases.

## Decision
**Chosen path: doc-edit + minimal dead-code cleanup.**

Rationale:
- `development-roadmap.md` lists music/geography as "lower priority / exploratory" — no near-term commitment.
- Wiring wordle through interface costs ~1d engineering with no shipped benefit; YAGNI.
- Renaming proto enum values now is cheap (no live external clients per arch review) — do it to avoid future `buf breaking` pain. **This is the only non-doc change.**

## Related Code Files
**Modify**
- `README.md` (line 12: drop "Pluggable game types — Wordle-style at launch; music, geography, image variants planned"; replace with "Wordle at launch")
- `docs/project-overview-pdr.md:17` (drop "shared/game.Game interface supports Wordle at launch")
- `docs/codebase-summary.md:61-63` (correct or remove interface description)
- `docs/development-roadmap.md` (move "pluggable" from any "current" section to explicit "v2 / not started")
- `docs/system-architecture.md` (drop `Game` interface diagram if any references it; pairs with Phase 07 H4)
- `proto/dleague/v1/envelope.proto:18-23` (rename `MESSAGE_TYPE_GAME_MOVE`→`MESSAGE_TYPE_WORDLE_MOVE`, same for STATE — comment that values 6/7 are wordle-only)
- `server/internal/ws/hub.go` (dispatch switch case rename)
- `server/internal/ws/match_handler.go`, `match_room.go`, `game_handler.go`, etc. (any `MessageType_GAME_MOVE` ref)
- `web/src/lib/ws.ts` and Phaser scenes (any `MessageType.GAME_MOVE` ref)
- Regenerate pb: `make proto-gen`
- `shared/game/game.go` (add top-comment scaffold note; drop unused `KeyEnter`/`KeyBackspace` consts at lines 38-40)
- `shared/game/registry.go` (add top-comment scaffold note)
- `server/internal/store/games.go` (delete file OR strip `TODO(phase-07)` comments)
- `server/cmd/api/main.go:95` (drop `_ = store.NewGameRepo(db)` if file deleted; otherwise leave)

**Delete (recommended)**
- `server/internal/store/games.go` — fully unused; kept only for the unimplemented TODOs.

## Implementation Steps

### Doc edits
1. `README.md:12` — replace bullet "Pluggable game types — Wordle-style at launch; music, geography, image variants planned" with "Wordle at launch — multi-game support is on the long-term roadmap, not in this release."
2. `docs/project-overview-pdr.md:17` — strike sentence; replace with "Wordle is the only shipped game; the `shared/game` package contains a reserved (currently inactive) scaffold for future games."
3. `docs/codebase-summary.md:61-63` — update interface description to actual `Init/Validate/Apply/IsTerminal/Result` shape, and prefix with "Reserved scaffold (v2)".
4. `docs/development-roadmap.md` — confirm "pluggable game types" sits under v2/exploratory; if any current-phase ref remains, demote it.

### Proto rename (preempt buf breaking)
5. `proto/dleague/v1/envelope.proto:18-23` — rename `MESSAGE_TYPE_GAME_MOVE` → `MESSAGE_TYPE_WORDLE_MOVE`, `MESSAGE_TYPE_GAME_STATE` → `MESSAGE_TYPE_WORDLE_STATE`. Keep numeric values 6/7. Add comment block: `// Wordle-specific. Future games introduce their own MESSAGE_TYPE_<GAME>_MOVE/STATE.`
6. Run `make proto-gen` to regen Go + TS.
7. Update Go references: grep `MessageType_MESSAGE_TYPE_GAME_(MOVE|STATE)` across server; rename. Likely sites: `hub.go`, `match_handler.go`, `match_room.go`, `game_handler.go`, `sync_match_handler.go`.
8. Update TS references: grep `MessageType\.GAME_(MOVE|STATE)` across `web/src`. Rename to `WORDLE_MOVE/STATE`.
9. `go build ./... && (cd web && npm run check)` — no errors.

### Dead-code cleanup
10. Delete `server/internal/store/games.go`. Remove `_ = store.NewGameRepo(db)` from `server/cmd/api/main.go:95`. Remove the `gameRepo` import if any.
11. `shared/game/game.go:38-40` — drop unused `KeyEnter`/`KeyBackspace` constants (comment says "compatibility with interactive key-driven games" but no code uses them).
12. Add top-of-file comment to `shared/game/game.go` and `shared/game/registry.go`:
    ```go
    // Package game holds a reserved scaffold for future multi-game support.
    // The current release ships only Wordle, which constructs wordle.Wordle
    // directly without going through this interface or registry. See
    // plans/260509-1331-improvement-plan/phase-04-pluggability-decision.md.
    ```

## Todo List
- [ ] Doc edits: README + PDR + codebase-summary + roadmap (steps 1-4)
- [ ] Proto rename + regen + Go/TS callsite updates (steps 5-9)
- [ ] Delete `store/games.go` + drop `_ =` line (step 10)
- [ ] Drop unused `KeyEnter/KeyBackspace` consts (step 11)
- [ ] Scaffold-note comments on `shared/game/*` (step 12)

## Success Criteria
- `grep -rni "pluggable" README.md docs/` returns only roadmap/v2 contexts, never present-tense claim.
- `go build ./...` green; `cd web && npm run check` green.
- `grep -r "MESSAGE_TYPE_GAME_MOVE\|MessageType_GAME_MOVE\|MessageType\.GAME_MOVE" .` returns 0 hits.
- `grep -r "TODO(phase-" .` returns 0 hits (closed phase TODOs cleaned).
- `shared/game/` files have scaffold-note comments OR are deleted (current plan: keep with notes).

## Risk Assessment
- **Proto rename breaks any external consumer:** none documented; `buf breaking` against `main` should be re-run as part of the phase to confirm. If the project gains external API consumers later, this is a free win taken now.
- **Delete `games.go` regression:** none — file is fully unused. Verified via grep.
- **Doc tone:** "v2 not in this release" must not contradict marketing; verify with PM before merge.

## Security Considerations
None.

## Next Steps
- Phase 07 picks up `system-architecture.md` dispatch table (H4) which depends on the renamed enum values from this phase.
- If team later commits to a 2nd game: separate plan to wire interface, add `oneof Payload` or `game_id` field, build the actual game. **Schema note (Phase 05 D-1):** `daily_puzzles._id` is currently date-only ("YYYY-MM-DD"). When adding the 2nd game, migrate to compound `_id` like `"<game>_<date>"` or add a new unique index on `(date, game_id)` to prevent collisions. See `phase-05-persistence-integrity.md` completion notes D-1.
