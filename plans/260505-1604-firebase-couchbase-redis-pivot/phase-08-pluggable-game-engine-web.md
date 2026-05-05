---
phase: 8
title: "Pluggable game-engine layer (web)"
status: pending
priority: P2
effort: "2d"
dependencies: [7]
---

# Phase 8: Pluggable game-engine layer (web)

## Context Links

- Plan: [plan.md](plan.md)
- Prior superseded phase: `260505-1407-firebase-platform-pivot/phase-5-pluggable-game-engine-web.md`

## Overview

Define a thin TypeScript interface so individual game variants (`-dle` games like Wordle, Sumdle, Quordle) can plug in without touching auth/networking. First implementation: Wordle-style daily puzzle. Spike Phaser 4 to gauge whether a canvas engine is needed.

## Key Insights

- Most `-dle` games are HTML/CSS, not canvas. Phaser is overkill unless we need real-time animation.
- The interface boundary keeps each game self-contained: receives input from `useAuth` + WS, renders state, dispatches attempts via callback.
- Daily puzzle is fully client-side once the puzzle doc is fetched; only attempts go back to server.

## Requirements

- Functional: `GameEngine` interface with `init`, `submitGuess`, `subscribe`, `dispose`. Wordle implementation as first concrete game.
- Non-functional: lazy-loadable per game variant; bundle split per variant.

## Architecture

```typescript
interface GameEngine<TState, TAction> {
  init(puzzle: Puzzle, prevAttempt?: Attempt): TState;
  step(state: TState, action: TAction): { state: TState; emit?: ServerEvent };
  isComplete(state: TState): boolean;
}

// React side:
<GameRunner engine={WordleEngine} puzzle={puzzle} onComplete={postAttempt} />
```

```
client/web/src/games/
├── engine.ts              # interface + types
├── wordle/
│   ├── engine.ts          # implements GameEngine
│   ├── Board.tsx          # render
│   └── Keyboard.tsx
└── runner/
    └── GameRunner.tsx     # generic shell
```

## Related Code Files

- Create:
  - `client/web/src/games/engine.ts`
  - `client/web/src/games/wordle/{engine.ts,Board.tsx,Keyboard.tsx,index.ts}`
  - `client/web/src/games/runner/GameRunner.tsx`
- Modify:
  - `client/web/src/pages/Lobby.tsx` → add "Play today's puzzle" button → renders `<GameRunner>`

## Implementation Steps

1. Define `GameEngine<TState,TAction>` interface in `games/engine.ts`.
2. Implement `WordleEngine`:
   - State: `{guesses: string[], evaluations: Eval[][], status: 'playing'|'won'|'lost'}`
   - Actions: `{type: 'guess', word: string}`
   - Pure logic, no I/O.
3. `Board.tsx` + `Keyboard.tsx` consume state via props.
4. `GameRunner.tsx`:
   - Fetches `GET /puzzles/:date` (REST) on mount
   - Restores previous attempt from `GET /attempts/me/:date` (resume in progress)
   - Wires keyboard input → `engine.step` → render
   - On complete, POST `/attempts` with final state
5. Phaser 4 spike: load Phaser in dev, render a smoke-test scene; if perceptible bundle/perf cost vs HTML/CSS, document and skip; else add as optional renderer for future games.

## Todo List

- [ ] GameEngine interface + types
- [ ] WordleEngine pure logic
- [ ] Board + Keyboard components
- [ ] GameRunner orchestrator
- [ ] Lobby integration
- [ ] Phaser 4 spike outcome documented
- [ ] Resume-in-progress flow works (refresh page mid-game)

## Success Criteria

- [ ] Player can complete a Wordle attempt end-to-end: load puzzle → guess until win/lose → result persists across refresh
- [ ] Adding a second game variant (`Sumdle`) is a copy-of-folder + register-in-runner exercise; no plumbing change
- [ ] Bundle split per game variant via Vite dynamic import

## Risk Assessment

- **Over-engineering the interface** — start with minimum viable; expand as second game lands.
- **Phaser eats bundle** — only adopt if a game variant genuinely needs canvas physics.

## Security Considerations

- Client-side state isn't trusted. Server re-validates the final attempt against the puzzle's solution before persisting score.

## Next Steps

Phase 9 wires the server side of `/puzzles/:date` and `/attempts` endpoints.

## Unresolved Questions

- Use Zustand or React Context for game state? Defer; either is fine.
- Animation library (framer-motion) needed? Probably yes for tile flips; defer to first variant landing.
