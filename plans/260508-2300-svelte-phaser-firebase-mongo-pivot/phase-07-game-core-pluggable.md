---
phase: 7
title: "Game core pluggable + Wordle (server-authoritative)"
status: completed
priority: P1
effort: 2w
dependencies: [6]
---

# Phase 07 — Game core pluggable + Wordle

## Context Links
- `plans/archive/260505-0947-dleague-pvp-game/phase-02-game-core-pluggable.md` (mine for spec; rebase Ebiten→Svelte+Phaser)
- `plans/reports/code-review-260508-2300-phase1-foundation.md` (L8: opaque `bytes State` + unresolved Q5 about JSON-vs-proto)
- `shared/game/{game.go,registry.go}` (Phase 1 stubs to generalize)
- `proto/dleague/v1/envelope.proto` (concrete `WordleState` replaces opaque `bytes`)
- `web/src/lib/phaser/scenes/title-scene.ts` (Phase 06)
- `server/internal/store/{games.go,daily_puzzles.go}` (skeleton from Phase 04)

## Overview
Build the pluggable `Game` interface in Go (`shared/game/`) and the parallel TS interface (`web/src/lib/game/`). Replace opaque `bytes State` in proto envelope with concrete typed messages (`WordleState`, `WordleMove`). Ship server-authoritative Wordle: target word never sent to client; client submits guess, server validates + returns colors + win/loss. Daily puzzle generator job seeds `daily_puzzles` collection.

## Key Insights
- **Server-authoritative validation:** target word stays on server. Client guesses → server scores letters (green/yellow/gray) → returns hints only. Closes the cheat surface left in Phase 1.
- **Concrete `GameState` message** ties wire format to game type. Replaces code-review L8 ambiguity. New oneof in envelope or separate message types per game.
- **Wordlist source:** open-source 2315-answer list (pre-NYT public). Stored in Mongo `wordlists` collection (research §3 mongo report recommendation, not embedded — avoids server rebuild for wordlist tweaks). Fallback embedded for first deploy.
- **Daily puzzle:** UTC-midnight seed → wordlist index. Generator job populates `daily_puzzles[YYYY-MM-DD]` with `{seed, solution_hash, solution}` (solution stored server-only; never returned to client).
- **Pluggable interface:** Go `Game` interface (Init/Validate/Apply/IsTerminal/Result) + TS counterpart for client-side optimistic preview. Server is source of truth.
- **Client UI:** Svelte `Board.svelte` for grid + on-screen keyboard; Phaser scene for color/flip animations.

## Requirements
**Functional:**
- Go interface `shared/game/game.go` (or `Game` re-exported): `Init(seed int64) State`, `Validate(state State, move Move) error`, `Apply(state State, move Move) (State, []Event)`, `IsTerminal(state State) bool`, `Result(state State) Result`.
- TS interface `web/src/lib/game/game.ts` mirrors Go shape.
- Wordle Go impl: `server/internal/game/wordle/`:
  - `wordle.go` — `New(answer string)`, `Validate`, `Apply`, `IsTerminal`, `Result`.
  - `colors.go` — green/yellow/gray algorithm handling repeated letters.
  - `wordlist.go` — load from Mongo `wordlists` collection; fallback to embedded `data/answers.txt`.
- Wordle TS impl: `web/src/lib/game/wordle/wordle.ts` (client-side preview only — never authoritative).
- Proto: replace opaque `bytes State` with typed messages. New messages in `proto/dleague/v1/wordle.proto`:
  - `WordleState{repeated string guesses; repeated WordleHint hints; int32 attempts_remaining; bool won; bool lost}`
  - `WordleMove{string guess}`
  - `WordleHint{repeated Color colors}` with enum `COLOR_GREEN/YELLOW/GRAY`.
  - Envelope payload now contains marshaled WordleMove for `MESSAGE_TYPE_GAME_MOVE` and WordleState for `MESSAGE_TYPE_GAME_STATE`.
- Server WS handlers: `server/internal/ws/handlers/game.go` — `handleGameMove(c *Conn, env *Envelope)` validates + applies + replies with new state. Match-state stash in-memory keyed by `userID + matchID` (durable persistence in Phase 08/09).
- Daily puzzle generator: `server/internal/game/wordle/daily.go` exposes `EnsureToday(ctx, repo, time.Now().UTC())` — reads `daily_puzzles[today]`; if absent, picks word via deterministic seed and inserts. Called by main.go on boot + by hourly tick (good enough; cron not needed).
- Client: Svelte `Board.svelte` (5×6 grid), `Keyboard.svelte` (on-screen QWERTY with letter-state), Phaser overlay for tile-flip animation on submit.
- Solo offline play (no PvP yet): client can play full daily round; submits each guess via WS; server records nothing yet (until Phase 08).

**Non-functional:**
- Each game file <200 LOC.
- Wordle Go test coverage >80%.
- Wordlist load <100ms (Mongo) with embedded fallback if collection empty.
- Tile-flip animation 60fps in Phaser.
- Server validation <5ms p95.

## Architecture
```
client                                            server
─────                                            ──────
solo play screen
  Board renders grid + Keyboard
  user types CRANE, presses Enter
    ↳ ws.sendRequest(GAME_MOVE, WordleMove{guess:"CRANE"})
                       ────────────►
                                                  handleGameMove:
                                                   ├─ validate guess (5 letters, in dictionary)
                                                   ├─ load today's solution from daily_puzzles
                                                   ├─ score colors
                                                   ├─ update in-memory state
                                                   ├─ check terminal
                                                   └─ reply WordleState
                       ◄────────────
  ws.onMessage GAME_STATE → eventBus.emit('state-update')
  PhaserScene plays tile-flip animation per letter color
  if state.won: navigate to results
```

`shared/game/` package layout:
```
shared/game/
├── game.go        (interface + types)
├── registry.go    (existing — generalized)
├── state.go       (State, Move, Event base types as proto wrappers)
└── wordle/        (sub-package; not in shared/ for server-only logic; client uses TS)
```

Server-authoritative trust: solution is **never** marshaled into `WordleState`. Only after `IsTerminal` does `Result.solution` field populate (post-game reveal).

## Related Code Files
**Create (Go):**
- `shared/game/state.go`
- `server/internal/game/wordle/wordle.go`
- `server/internal/game/wordle/colors.go`
- `server/internal/game/wordle/wordlist.go`
- `server/internal/game/wordle/daily.go`
- `server/internal/game/wordle/wordle_test.go`
- `server/internal/game/wordle/colors_test.go`
- `server/internal/game/wordle/data/answers.txt` (embedded fallback)
- `server/internal/game/wordle/data/dictionary.txt` (valid-guess list)
- `server/internal/ws/handlers/game.go` — `handleGameMove`, `handleGameState`
- `server/internal/store/wordlists.go` — repo for wordlist storage

**Create (TS / Svelte):**
- `web/src/lib/game/game.ts` — TS interface
- `web/src/lib/game/wordle/wordle.ts` — client preview
- `web/src/lib/game/wordle/colors.ts`
- `web/src/lib/components/board.svelte`
- `web/src/lib/components/keyboard.svelte`
- `web/src/lib/phaser/scenes/wordle-scene.ts` — tile-flip overlay
- `web/src/routes/play/+page.svelte` (solo daily play)

**Create (proto):**
- `proto/dleague/v1/wordle.proto`

**Modify:**
- `shared/game/game.go` — generalize types; remove `State = []byte` alias (code-review L8).
- `proto/dleague/v1/envelope.proto` — add `MESSAGE_TYPE_GAME_MOVE`, `MESSAGE_TYPE_GAME_STATE`.
- `server/internal/ws/hub.go` — register new dispatch cases.
- `server/internal/store/daily_puzzles.go` — fill `EnsureToday`, `GetByDate`.
- `server/cmd/server/main.go` — boot calls `wordle.EnsureToday` on startup.
- `web/src/routes/+page.svelte` — title nav adds "Play Daily" button → `/play`.

**Delete:** none.

## Implementation Steps
1. **Proto schema:** new `wordle.proto`; new envelope enum values `GAME_MOVE = 7`, `GAME_STATE = 8`. `make proto-gen` → emits Go + TS.
2. **Generalize `shared/game/`:** replace `State = []byte` with proper interface; document that game implementations marshal/unmarshal their own state into Envelope payload. Game interface: `Validate`, `Apply`, `IsTerminal`, `Result`.
3. **Wordle server logic (`wordle.go`):** struct with `solution string`, `guesses []string`, `hints []Hint`, `attemptsRemaining int`. `Validate(guess)` checks length + dictionary. `Apply(guess)` appends + scores + marks terminal. `Score(guess, solution)` is the green/yellow/gray algorithm with double-letter handling.
4. **Colors (`colors.go`):** classic two-pass algorithm. Test against published edge cases (e.g., guess `ALLEE` vs solution `EERIE` → only one yellow E, etc.).
5. **Wordlist (`wordlist.go`):** `LoadAnswers(ctx, repo) []string` queries `wordlists.collection.find({"type":"wordle_answers"})`; if empty, falls back to `//go:embed data/answers.txt`. Same for `LoadDictionary`.
6. **Daily generator (`daily.go`):** `EnsureToday(ctx, dailyRepo, wordlistRepo, now time.Time)`:
   - Compute date string `YYYY-MM-DD` UTC.
   - `daily_repo.GetByDate(date)` → if exists, return.
   - Compute deterministic seed: `sha256(date + "wordle-v1")` → `int64` mod len(answers).
   - Insert `{_id: date, seed, solution: answers[idx], solution_hash: sha256(solution)}`.
   - Boot calls; hourly tick optional.
7. **WS handler (`handlers/game.go`):**
   - In-memory match state: `map[userID]*WordleSession` (single solo session per user; PvP added in Phase 08+09).
   - On `GAME_MOVE`: validate user authenticated; deserialize `WordleMove`; load today's solution from `dailyRepo`; instantiate `wordle.Wordle` with state; `Validate` + `Apply`; serialize `WordleState` (omit solution unless terminal); reply.
8. **Hub wiring:** register new message types in dispatcher.
9. **Wordlists collection:** seed once via `make seed-wordlists` (script reads embedded `data/answers.txt` + uploads). Boot can also seed if collection empty.
10. **TS Wordle (client):** mirror server scoring for instant local preview (UI feedback before server response). Authoritative state always comes from server reply.
11. **Svelte Board:** 5×6 grid; props `{guesses, hints, currentInput}`. Subscribe to game state from WS message.
12. **Svelte Keyboard:** virtual keyboard component; tracks letter-state colors as game progresses.
13. **PhaserScene wordle-scene:** overlay on Svelte board with tile-flip animation (Y-axis rotation 0°→90°→colorized→90°→0°). EventBus event `wordle:flip-row` triggered after server response.
14. **Solo route `/play/+page.svelte`:** wires WS, sends GAME_MOVE on submit, listens GAME_STATE, renders Board + Keyboard + PhaserScene overlay.
15. **Tests:**
    - `wordle_test.go`: full happy path, all-correct, all-miss, repeated-letter edge cases, invalid guess length, dictionary miss.
    - `colors_test.go`: 10+ canonical edge cases.
    - `daily_test.go`: same date → same solution; sequential days produce different solutions (with high probability for 2315-word list).
    - TS: `wordle.test.ts` mirrors Go cases via Vitest.
16. **Manual smoke:** sign in → /play → submit `CRANE` → see colors + animation. Submit through to win or 6 attempts → results screen.

## Todo List
- [x] Proto: wordle.proto + envelope enum entries
- [x] Generalize shared/game (drop bytes State)
- [x] Wordle Go: wordle.go + colors.go
- [x] Wordlist loader (Mongo + embed fallback)
- [x] Daily puzzle generator
- [x] WS handler `handleGameMove`
- [x] In-memory user session map
- [x] Wordlists seed script
- [x] TS Wordle client preview
- [x] Svelte Board + Keyboard
- [x] Phaser tile-flip scene
- [x] /play route
- [x] Go tests >80% wordle (84.9%)
- [x] TS tests for color algorithm (9 Vitest cases)
- [ ] Manual smoke (requires running Mongo + Firebase emulator)
- [x] Update docs/system-architecture.md (game flow)

## Success Criteria
- [ ] User plays full daily Wordle in browser → wins or loses correctly
- [ ] Identical date across users → identical solution
- [ ] Solution never appears in WS frames until terminal state
- [ ] Color scoring correct on repeated-letter cases (test passes)
- [ ] Adding a hypothetical second game requires 0 changes to UI shell (just new game pkg + proto + scene)
- [ ] Wordle Go test coverage ≥80%
- [ ] Server validation <5ms p95 under solo load

## Risk Assessment
| Risk                                                   | Likelihood | Impact | Mitigation                                                       |
|--------------------------------------------------------|------------|--------|------------------------------------------------------------------|
| Color algorithm wrong on repeated-letter edge cases    | Medium     | High   | Comprehensive test table from canonical sources.                 |
| Solution leaks via timing / hint pattern               | Low        | Medium | Constant-time response; no early exit on validate fail.          |
| Wordlist licensing                                     | Low        | High   | Use pre-2022 NYT-leaked Wordle list (public domain) or generate own from open dictionary. |
| In-memory session lost on server restart               | High       | Low    | Phase 08 makes durable; for solo OK to forfeit.                  |
| TS preview diverges from Go truth                       | Medium     | Low    | Server reply always overrides preview; document as best-effort.  |
| Phaser animation jank under low-power devices          | Medium     | Low    | Fall back to CSS transitions if `prefers-reduced-motion`.        |

## Security Considerations
- Solution never serialized into pre-terminal `WordleState`.
- Server re-validates dictionary on every guess; client cannot smuggle non-words.
- `solution_hash` stored alongside solution for audit (verify post-mortem that solution wasn't tampered).
- Daily seed deterministic; document algorithm so suspected cheaters can be reproduced offline.
- No XSS surface (Svelte auto-escapes; only structured proto data on wire).

## Next Steps
- Phase 08 — Async PvP — depends on Wordle game core + daily puzzle infrastructure shipped here.
