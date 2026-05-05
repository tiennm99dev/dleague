---
phase: 8
title: "Pluggable game variants on Phaser 4 (Svelte HUD)"
status: pending
priority: P1
effort: "2d"
dependencies: [7]
---

# Phase 8: Pluggable game variants on Phaser 4

## Context Links

- Plan: [plan.md](plan.md)
- Engine fitness: [researcher-260505-1728-dle-platform-engine-fitness.md](../reports/researcher-260505-1728-dle-platform-engine-fitness.md)
- Phaser Scenes docs: https://docs.phaser.io/phaser/concepts/scenes

## Overview

Define the per-variant interface so individual `-dle` games (Wordle, Sumdle, Quordle, etc.) plug in as **Phaser 4 scenes** with **Svelte HUDs** without touching auth/networking. First implementation: Wordle-style daily puzzle. Each variant lives under `src/games/<name>/`.

## Key Insights

- Phaser **Scenes** are the natural primitive for "one game per scene" — register all variants on the Phaser game instance, switch via `scene.start(<key>)`.
- Svelte components handle the HUD (score, attempt counter, win/lose modal) overlaid on the Phaser canvas via the EventBus from Phase 7.
- Pure-canvas Wordle = animated tile flips, color reveals, particle wins → Phaser shines here vs DOM/CSS.
- DOM accessibility: provide a hidden `<input>` for screen readers + keyboard input that proxies to Phaser (covered by template's input handling pattern).

## Requirements

- Functional: `GameVariant` interface with `key`, `Scene` (Phaser), `Hud` (Svelte component); Wordle implementation as first concrete game.
- Non-functional: lazy-loadable per variant via Vite dynamic import; Phaser bundle weight stays single (~345 KB) even with N variants.

## Architecture

```typescript
// games/types.ts
interface GameVariant {
  key: string;                    // 'wordle', 'sumdle', etc.
  Scene: typeof Phaser.Scene;     // the Phaser scene class
  Hud: typeof SvelteComponent;    // the HUD Svelte component
  meta: { title: string; difficulty: 'easy'|'medium'|'hard'; tagline: string };
}
```

```
client/web/src/games/
├── types.ts                  # GameVariant interface
├── registry.ts               # all variants registered here (lazy imports)
├── runner/
│   ├── GameRunner.svelte     # generic shell: loads variant, mounts Phaser scene + HUD
│   └── eventbus-helpers.ts   # typed wrappers around the shared EventBus
└── wordle/
    ├── WordleScene.ts        # Phaser scene: grid, keyboard, tile flips, win effects
    ├── WordleHud.svelte      # HUD: score + attempts left + win/lose modal
    ├── scoring.ts            # pure scoring func (server re-validates)
    └── index.ts              # exports GameVariant
```

Flow on play:
```
Lobby → user clicks "Play today's puzzle"
  → GameRunner loads `wordle` variant via dynamic import
  → fetches today's puzzle via REST (Phase 9)
  → fetches resume-state via /attempts/me/:date if exists
  → registers WordleScene with Phaser game
  → mounts WordleHud as Svelte sibling
  → scene.start('wordle')
  → on win/lose → EventBus.emit('attempt-complete', result)
  → GameRunner POSTs /attempts (Phase 9)
```

## Related Code Files

- Create:
  - `client/web/src/games/types.ts`
  - `client/web/src/games/registry.ts`
  - `client/web/src/games/runner/{GameRunner.svelte,eventbus-helpers.ts}`
  - `client/web/src/games/wordle/{WordleScene.ts,WordleHud.svelte,scoring.ts,index.ts}`
- Modify:
  - `client/web/src/routes/Lobby.svelte` → add "Play today's puzzle" button → renders `<GameRunner variantKey="wordle" />`

## Implementation Steps

1. Define `GameVariant` interface in `games/types.ts`.
2. `registry.ts`: `Map<string, () => Promise<GameVariant>>` for lazy-loaded variants.
3. Implement `WordleScene` (Phaser):
   - Grid: 6 rows × 5 columns of tiles
   - Keyboard: 26 keys + Enter + Backspace
   - Animations: tile flip on submit, color reveal, win confetti, lose shake
   - State: `{guesses, evaluations, status}` — pure logic in `scoring.ts` (unit-tested), Phaser scene reads it
   - Emit `attempt-complete` on win/lose with full state
4. `WordleHud.svelte`: shows attempts remaining, current row, win/lose modal with score + share button.
5. `GameRunner.svelte`:
   - Props: `variantKey`
   - On mount: fetch puzzle via REST, fetch resume-state, dynamic import variant, register scene, mount HUD, start scene
   - Listen for `attempt-complete` → POST `/attempts` → emit `done` event
6. `Lobby.svelte`: add CTA → `<GameRunner variantKey="wordle" />`

## Todo List

- [ ] `GameVariant` interface + `registry.ts`
- [ ] Wordle pure scoring func + unit tests
- [ ] `WordleScene` (Phaser): grid + keyboard + animations + EventBus emits
- [ ] `WordleHud.svelte`: attempts indicator + win/lose modal
- [ ] `GameRunner.svelte` orchestrator
- [ ] Lobby integration
- [ ] Resume-in-progress flow works (refresh page mid-game preserves state)
- [ ] Adding a second variant = copy `wordle/` folder + register + done (no plumbing change)

## Success Criteria

- [ ] Player completes a Wordle attempt end-to-end: load → guess until win/lose → result persists via REST → leaderboard updates
- [ ] Tile flips animate cleanly at 60 fps on mid-tier mobile (Capacitor WebView)
- [ ] Adding a stub `Sumdle` variant (different rules, same shape) takes <30 min — proves the seam works
- [ ] Bundle: variant lazy-load works (only `wordle/` chunks load until played)

## Risk Assessment

- **Phaser scene churn** — switching between variants must clean up scenes properly (no leaked tweens/timers). Test by hammering the back-to-lobby button.
- **Mobile WebView frame drops** — Android WebView can stutter on heavy animation. Mitigation: use Phaser 4 SpriteGPULayer for tile flips if needed; fall back to simpler easings.
- **Accessibility** — canvas is opaque to screen readers. Mitigation: maintain a parallel hidden DOM mirror of state for SR users (Wordle's official site does this).

## Security Considerations

- Client-side state isn't trusted. Server re-validates the final attempt against the puzzle's solution before persisting score.

## Next Steps

Phase 9 wires server-side `/puzzles/:date`, `/attempts`, `/leaderboards` endpoints (already planned).

## Unresolved Questions

- DOM-mirror strategy for accessibility — depth: full state mirror, or only "current word entered + result"? Defer; ship Wordle, then add a11y pass.
- Animation library — Phaser 4 has built-in tweens; avoid GSAP unless complexity demands it.
