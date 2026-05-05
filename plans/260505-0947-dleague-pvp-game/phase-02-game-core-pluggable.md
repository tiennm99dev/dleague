---
phase: 2
title: "Game core (pluggable -dle)"
status: pending
priority: P1
effort: 2w
dependencies: [1]
---

# Phase 2: Game core (pluggable -dle)

## Overview

Build the pluggable `Game` interface and ship the first concrete game: Wordle-style 5-letter word guessing. Single-player offline only — no backend yet. Architecture must accept future games (music, geography, image) without rewriting client core.

> **Porting reference:** Tile animation pattern adapted from `hajimehoshi/ebiten/examples/2048/2048/tile.go` (Apache-2.0). See `reports/xia-260505-1014-core-extraction.md` for full mapping.

## Requirements

- **Functional:**
  - `Game` interface in `shared/game/` with lifecycle: `Init(seed) / HandleKey(key) / Tick() / Render(screen) / State() GameState`
  - `WordleGame` concrete impl: 6 attempts, 5-letter answer, color feedback (green/yellow/gray)
  - HTML overlay handles keyboard input (responsive, mobile-friendly), bridges to Go via `syscall/js`
  - Ebitengine canvas renders animations: tile flip, shake on invalid, win/lose effect
  - Game state serializable to JSON (for later async PvP sync)
  - Wordlist embedded via `go:embed` (start with 2315 Wordle answer list, public domain)
  - Title screen → game scene → results scene flow
- **Non-functional:**
  - Game logic 100% in `shared/` so server can validate identical results
  - Each game file <200 LOC, split scenes/components
  - 60 FPS render, low CPU

## Architecture

**Pluggable game pattern:**

```go
// shared/game/game.go
type Game interface {
    Init(seed int64) error
    HandleKey(k Key) StateChange
    Tick(dt time.Duration)
    State() State
    IsTerminal() bool
    Result() Result
}

type Registry struct { games map[string]Factory }
func Register(id string, f Factory) // wordle, music, geo register here
```

**Client flow:**
- HTML overlay captures keystrokes (better UX than canvas keyboard) → posts events to WASM via `js.Global().Get("dleagueEvent")`
- WASM forwards to active `Game.HandleKey`
- Game returns StateChange → scene re-renders canvas
- Hybrid: text grid uses HTML for accessibility; canvas adds visual flair (tile flip animations, particles)

**Daily seed:** UTC midnight + game-id → deterministic seed. Server can validate same daily produces same answer.

## Related Code Files

**Create:**
- `shared/game/game.go` (interface, types: Key, State, Result)
- `shared/game/registry.go`
- `shared/game/wordle/wordle.go` (logic, <200 LOC)
- `shared/game/wordle/wordle_test.go`
- `shared/game/wordle/wordlist.go` (`//go:embed answers.txt`)
- `shared/game/wordle/data/answers.txt`, `data/dictionary.txt`
- `client/internal/scene/title.go`
- `client/internal/scene/game.go`
- `client/internal/scene/results.go`
- `client/internal/ui/keyboard_overlay.go` (syscall/js bridge)
- `client/internal/scene/wordle/tile.go` — port Tile struct + counter-based animation from ebiten/2048/tile.go (Apache-2.0 header preserved). Replace movingCount/poppingCount semantics with flipCount/shakeCount for Wordle reveal animation
- `client/internal/scene/wordle/board.go` — port grid composition pattern from ebiten/2048/board.go. 5×6 fixed grid (vs 2048's 4×4 dynamic)
- `client/internal/scene/wordle/colors.go` — Wordle palette (green/yellow/gray) — inspired by 2048/colors.go pattern, original implementation
- `client/internal/scene/wordle/input.go` — keyboard mapping, port pattern from ebiten/2048/input.go
- `web/index.html` (extend with input overlay div)
- `web/styles.css` (overlay grid styling)

**Modify:**
- `client/cmd/web/main.go` (scene manager wiring)

## Implementation Steps

1. Define `Game` interface + `State`/`Result`/`Key` types in `shared/game/`
1b. Port Ebitengine Tile pattern: `current/next` data + counter-based `Update()` decrements + `Draw()` interpolation via linear blend (`mean()`). Replace 2048's `movingCount` (translation) + `poppingCount` (scale pop) with `flipCount` (Y-axis rotation 0°→90°→hide→90°→0° with new color) + `shakeCount` (X-axis offset oscillation on invalid word). Keep Apache-2.0 copyright header at top of `tile.go`.
2. Build `WordleGame`: word validation, attempt tracking, hint computation (green/yellow/gray)
3. Embed wordlists (answers ~2315, dictionary ~10k for valid-guess check)
4. Unit-test `WordleGame` exhaustively (correct guess, partial, repeated letters edge case, invalid word)
5. HTML overlay: 5×6 input grid styled to match canvas position; on-screen keyboard for mobile
6. JS↔WASM bridge: postMessage events, `dleagueEvent({type:'key', value:'A'})`
7. Ebitengine scene manager: title → game → results, with transitions
8. Tile-flip animation on attempt submit (canvas overlay on HTML cell)
9. Shake animation on invalid word
10. Win/lose results scene with share button (placeholder, hooks up Phase 4)
11. Daily seed: `time.Now().UTC().Format("2006-01-02")` → SHA256 → int64 → wordlist index
12. Hook a `?seed=` URL param for testing reproducibility

## Todo List

- [ ] `Game` interface + types
- [ ] Registry pattern
- [ ] `WordleGame` logic + tests
- [ ] Embedded wordlists
- [ ] Title/game/results scenes
- [ ] HTML keyboard overlay (desktop + mobile)
- [ ] JS↔WASM event bridge
- [ ] Tile flip + shake animations
- [ ] Daily seed generator
- [ ] Test coverage >80% for `wordle/`

## Success Criteria

- [ ] User plays full Wordle round offline: type letters → submit → see colored feedback → win/lose at 6 attempts
- [ ] Mobile browser: on-screen keyboard works, no zoom-on-input glitch
- [ ] Same date always produces same daily answer
- [ ] Adding a hypothetical second game requires 0 changes to scene/UI code (just register + new Game impl)
- [ ] Wordle test coverage >80%
- [ ] WASM bundle <10MB

## Risk Assessment

| Risk | Mitigation |
|------|-----------|
| HTML overlay z-index/position drift on canvas resize | CSS Grid with viewport units; resize listener syncs canvas + overlay |
| `syscall/js` performance for high-frequency events | Only forward keydown, not every tick; debounce |
| Daily seed timezone confusion | Always UTC; document; show "next puzzle in" timer in user's TZ |
| Wordlist licensing | Use NYT Wordle's pre-2022 leaked list (now public) OR generate own from open dictionary |
| Game interface too rigid for music/geo games | Keep interface minimal (Init/HandleInput/Render/State); media-specific concerns inside game impl |

## Security Considerations

- Wordlist embedded in WASM = visible to client (fine for offline; server re-validates Phase 4)
- No user input goes to backend yet; XSS not relevant
