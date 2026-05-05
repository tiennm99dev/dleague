# Phase 5: Pluggable game engine (TypeScript)

## Context Links
- Parent (superseded): `plans/260505-0947-dleague-pvp-game/phase-02-game-core-pluggable.md` — original Go-side pluggable interface
- Phase-2 wire: `MESSAGE_TYPE_GAME_GUESS`, `MESSAGE_TYPE_GAME_FEEDBACK` (added this phase)
- Phase-3 schema: `/puzzles/{date}_{gameId}`, `/server_only/answers/{date}_{gameId}` (private)
- Locked: server-authoritative validation; client renders only

## Overview
- **Priority:** P1 (core gameplay)
- **Status:** pending
- **Effort:** 4d
- TS port of pluggable `-dle` game interface. Wordle-style game = first concrete impl. Server holds answers; client sends guess; server returns feedback. Same `Game` interface in TS (client side) and Go (server side) so future games (music, geography) plug in identically.

## Key Insights
- "Pluggable" at MVP = ONE game (Wordle); the interface exists so adding `loldle` doesn't refactor core. YAGNI: don't ship 2 games; ship 1 + a `Game` interface
- Client receives only `feedback` (color codes per tile), NEVER the answer; even on game end, server reveals answer via dedicated message — and only after all attempts used or both players done (sync mode)
- Wordle answer pool = 2315 open-source list; ship as `server/internal/games/wordle/wordlist.txt` (compile-time embed)
- Daily puzzle generation: deterministic from `(puzzle_date, game_id)` via PRNG seeded with `sha256(date+game_id)`; same answer for everyone that day
- Guess validation = 2 checks: (1) is the guess in the valid-guess wordlist (~12k words; bigger than answer pool); (2) compare to answer for feedback. Both server-side
- Feedback encoding: 1 char per tile — `G` (green/correct), `Y` (yellow/wrong-pos), `B` (gray/absent). String of length=word_length

## Requirements

### Functional

#### Wire format additions
- `MESSAGE_TYPE_GAME_START = 6` — client requests today's puzzle for a game_id
- `MESSAGE_TYPE_GAME_PUZZLE = 7` — server returns puzzle metadata (NOT answer)
- `MESSAGE_TYPE_GAME_GUESS = 8` — client submits a guess
- `MESSAGE_TYPE_GAME_FEEDBACK = 9` — server returns per-tile feedback
- `MESSAGE_TYPE_GAME_END = 10` — server reveals answer + final state

```proto
message GameStart { string game_id = 1; string match_id = 2; }
message GamePuzzle { string game_id = 1; string puzzle_date = 2; int32 word_length = 3; int32 max_attempts = 4; int32 attempts_remaining = 5; }
message GameGuess { string match_id = 1; string guess = 2; }
message GameFeedback { string match_id = 1; string guess = 2; string feedback = 3; int32 attempts_remaining = 4; bool won = 5; }
message GameEnd { string match_id = 1; string answer = 2; bool won = 3; int32 attempts_used = 4; }
```

#### Server `Game` interface (Go)
```go
// server/internal/games/game.go
type Game interface {
    ID() string                                       // "wordle"
    WordLength() int                                  // 5
    MaxAttempts() int                                 // 6
    GenerateAnswer(seed int64) string                 // deterministic from seed
    ValidateGuess(guess string) error                 // wordlist check; returns ErrInvalidGuess
    ComputeFeedback(answer, guess string) string      // "GBYBG" etc.
}
```
Wordle implementation: `server/internal/games/wordle/wordle.go`.

#### Client `Game` interface (TS)
```ts
// web/src/games/game.ts
export interface Game {
  id: string;
  wordLength: number;
  maxAttempts: number;
  // Pure render helpers; no answer logic client-side
  isValidLocalShape(guess: string): boolean;   // alphabetic, length match
  formatFeedback(feedback: string): TileColor[]; // 'GBY' → tile color array
}
```
Wordle TS impl: `web/src/games/wordle/wordle.ts`.

### Non-functional
- Wordle answer wordlist embedded via Go `embed` (~25 KB)
- Wordle valid-guess wordlist embedded (~120 KB; only on server)
- Client knows nothing about wordlists (can't cheat)
- Each game implementation in its own subpackage; <200 LOC per file

## Architecture

### Server-side flow
```
client → AUTH_HELLO → AUTH_ACK
client → GAME_START{game_id, match_id}
                     server: load /puzzles/{today}_{game_id} (or generate + write if missing)
                     server: read /server_only/answers/{today}_{game_id} (Admin SDK; never sent)
                     server: write /matches/{match_id}/attempts/{uid} = {attempts_used: 0, guesses: []}
server → GAME_PUZZLE{word_length, max_attempts, attempts_remaining: 6}

LOOP per guess:
client → GAME_GUESS{match_id, guess: "CRANE"}
                     server: validate guess (wordlist + auth + match status)
                     server: compute feedback against answer
                     server: increment attempt count; append guess+feedback to attempt doc
                     server: check win: feedback == "GGGGG"
                     server: check loss: attempts_remaining == 0
server → GAME_FEEDBACK{guess, feedback, attempts_remaining, won}
                     IF won OR loss:
                       server: mark match completed_at, status=completed
                       server: trigger leaderboard update (in phase-6)
server → GAME_END{answer, won, attempts_used}
```

### Files to create

#### Server (Go)
- `server/internal/games/game.go` — `Game` interface (~50 LOC)
- `server/internal/games/registry.go` — `Registry{games: map[string]Game}; Register; Get` (~40 LOC)
- `server/internal/games/registry_test.go` (~50 LOC)
- `server/internal/games/wordle/wordle.go` — concrete impl (~120 LOC)
- `server/internal/games/wordle/wordle_test.go` — unit + golden test on feedback (~150 LOC)
- `server/internal/games/wordle/answers.txt` — 2315-word answer pool (embedded)
- `server/internal/games/wordle/allowed.txt` — ~12k valid-guess pool (embedded)
- `server/internal/games/wordle/embed.go` — `//go:embed` declarations
- `server/internal/ws/handlers/game_start.go` — handles GAME_START (~80 LOC)
- `server/internal/ws/handlers/game_guess.go` — handles GAME_GUESS (~100 LOC)
- `server/internal/ws/handlers/game_test.go` — table-driven (~150 LOC)
- `server/internal/firestore/puzzles.go` — `GetOrCreatePuzzle(ctx, date, gameID)` (~80 LOC)
- `server/internal/firestore/attempts.go` — `AppendGuess`, `MarkComplete` (~100 LOC)
- `server/internal/firestore/answers.go` — `GetAnswer(ctx, date, gameID)` reading `/server_only/answers/*` (~50 LOC)

#### Client (TS)
- `web/src/games/game.ts` — `Game` interface
- `web/src/games/registry.ts` — `gameRegistry: Map<string, Game>`
- `web/src/games/wordle/wordle.ts` — concrete TS impl
- `web/src/components/game-grid.tsx` — renders tile grid (~120 LOC)
- `web/src/components/keyboard.tsx` — virtual keyboard with letter-state (~80 LOC)
- `web/src/components/game-screen.tsx` — wires WS messages → grid state (~150 LOC)
- `web/src/hooks/use-game.ts` — game state machine (~100 LOC)

### Files to modify
- `server/internal/ws/hub.go` — register handlers for GAME_* types
- `proto/dleague/v1/envelope.proto` — add 5 new MessageType enum values + 5 messages; regen
- `web/src/App.tsx` — render `<GameScreen/>` when authed
- `web/src/ws/client.ts` — add typed `send()` helpers per message type

## Implementation Steps

### Server
1. Extend `envelope.proto` with 5 new message types; `make proto-gen`
2. Create `games/game.go` interface
3. Create `games/registry.go` with thread-safe map
4. Create `games/wordle/`:
   - Embed `answers.txt` and `allowed.txt`
   - `GenerateAnswer(seed)` = `answerPool[seed % len(answerPool)]`
   - `ValidateGuess(guess)` = case-insensitive lookup in `allowed.txt` set
   - `ComputeFeedback(answer, guess)` — standard 2-pass Wordle algorithm:
     1. Pass 1: mark all G (exact match)
     2. Pass 2: count remaining letters in answer; mark Y if available, else B
   - **Golden test:** 20+ test cases including duplicate-letter edge cases (e.g. answer=ALLOY guess=LULLS → "BYBBB" not "YYBBB")
5. Create `firestore/puzzles.go`:
   - `GetOrCreatePuzzle`: `Get` doc; if `NotFound`, generate seed=`hash(date+gameID)`, get answer from registry, write `/puzzles/*` (public) + `/server_only/answers/*` (private)
6. Create `firestore/attempts.go`:
   - `AppendGuess`: transaction read → mutate guesses[]+feedback[] → write
   - `MarkComplete`: set status, completed_at, won
7. Create WS handlers:
   - `game_start.go`: read user uid from conn, get-or-create puzzle, init attempt doc, return GAME_PUZZLE
   - `game_guess.go`: load match + attempt + answer; validate guess against wordlist; compute feedback; persist; return GAME_FEEDBACK; if terminal, also send GAME_END
8. Wire hub dispatch for GAME_START + GAME_GUESS
9. Test: `make test`

### Client
1. Generate updated TS protobuf types via `proto-gen-ts`
2. Create `games/game.ts` + `games/wordle/wordle.ts`
3. Create `<GameGrid/>`, `<Keyboard/>`, `<GameScreen/>`
4. Wire `use-game.ts` hook:
   - State: current_guess (string), guesses (string[]), feedbacks (string[]), attempts_remaining
   - Actions: `submit(guess)` → ws.send(GAME_GUESS); on GAME_FEEDBACK → append; on GAME_END → show modal
5. Mount `<GameScreen/>` in `<App/>` when `wsState === 'READY'`

## Todo List

### Server
- [ ] Add 5 message types to envelope.proto
- [ ] Run proto-gen
- [ ] Create games/game.go interface
- [ ] Create games/registry.go + tests
- [ ] Embed answers.txt + allowed.txt under games/wordle/
- [ ] Implement Wordle game (GenerateAnswer, ValidateGuess, ComputeFeedback)
- [ ] Golden tests for feedback algorithm (esp. duplicate letters)
- [ ] Create firestore/puzzles.go + answers.go + attempts.go
- [ ] Implement game_start + game_guess WS handlers
- [ ] Wire hub dispatch
- [ ] go build + golangci-lint + make test

### Client
- [ ] Regen protobuf-ts types
- [ ] Create web/src/games/game.ts + wordle/wordle.ts
- [ ] Implement <GameGrid/>, <Keyboard/>, <GameScreen/>
- [ ] Implement use-game hook
- [ ] Wire WS message handlers
- [ ] Mount in App when authed
- [ ] Smoke test: full Wordle game flow

## Success Criteria
- [ ] Server generates same answer for same `(date, game_id)` deterministically
- [ ] Wordlist validation: invalid guess returns error, doesn't increment attempt count
- [ ] Feedback golden tests: all 20+ cases pass (incl. dup-letter cases)
- [ ] Client renders 6×5 grid; tile colors update on GAME_FEEDBACK
- [ ] Win flow: GAME_END shows modal with answer + duration + retry CTA
- [ ] Loss flow: same modal, sad path
- [ ] Answer NEVER appears in client bundle, network frame before GAME_END, or Firestore client-readable doc
- [ ] All Go files <200 LOC, all TS files <200 LOC
- [ ] Test coverage >80% on `games/` and handlers

## Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Duplicate-letter feedback algorithm wrong (common bug) | High | Med | Golden tests upfront; reference Wordle's published rules |
| Wordlist licensing | Low | Med | Use the public Wordle 2315-answer + 12k-allowed lists (open / public domain per common sources); cite in NOTICE |
| Embed files inflate server binary | Low | Low | ~150 KB total; trivial |
| Race on `AppendGuess` if user double-clicks submit | Med | Low | Firestore transaction enforces atomicity; idempotent on identical guess |
| Client cheats by submitting answer-hash brute force | N/A | N/A | Answer never client-side; cannot cheat |
| Server holds answer in Firestore — admin-SDK leak risk | Low | High | `/server_only/*` rules deny ALL client access; admin creds only |
| Wordlist mismatch between client UI hints and server validation | Med | Low | Client doesn't pre-validate against wordlist (server is source of truth); UI shows error after submission |

## Security Considerations
- Answer NEVER in any client-readable place (bundle, Firestore public docs, WS frame before GAME_END)
- `/server_only/answers/*` Firestore rule = `allow read,write: if false` (admin SDK bypasses)
- Server validates guess length + wordlist BEFORE computing feedback (prevent oracle attack via short guesses)
- Server validates `match_id` belongs to authenticated `uid` (creator or joiner)
- Rate-limit: max 1 GAME_GUESS per 500ms per conn (prevent brute force exhaustion of attempts) — implement in `conn.go` token bucket; cheap defense
- GAME_END only sent ONCE per match-attempt; idempotent if client retries

## Next Steps
- **Unblocks:** phase-6 (async PvP wires same handlers; match doc orchestrates 2 attempts)
- **Unblocks:** phase-7 (sync PvP broadcasts opponent guesses live)

## Unresolved Questions
1. Should attempts_used start at 0 or 1 in display? UI convention: "X / 6" tries — confirm in UX
2. Should clients see opponent's guess COUNTS in async (without seeing words)? Defer to phase-6 UX
3. Hard mode (must reuse green letters)? Defer to v2
4. Time-attack variant? Out of scope per parent plan
5. Should we ship `loldle` stub at MVP to prove pluggability, or wait? Recommend: just register Wordle; pluggable proof = the interface itself, not a second impl
