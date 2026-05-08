# Design Guidelines

**Status:** skeleton — populated in Phase 06 (client scaffold) and Phase 07 (game UI).

## UI principles
TODO Phase 06.
- Mobile-first responsive web (no native app yet).
- Calm palette; high contrast for color-blind accessibility on Wordle tiles.
- Keyboard-first input. On-screen keyboard for touch devices.
- Animations <250 ms (Phaser tweens).

## Component library
TODO Phase 06. Likely shadcn-svelte or hand-rolled.

## Color tokens
TODO Phase 06. Wordle tile colors must respect color-blind variants (e.g., daltonic mode toggle).

## Typography
TODO Phase 06.

## Iconography
TODO Phase 06. lucide-svelte or similar.

## Phaser scene conventions
TODO Phase 07.
- Single `BootScene` → `TitleScene` → `GameScene`.
- EventBus pattern (per `phaserjs/template-svelte`) for Svelte ↔ Phaser communication.
- Scene cleanup on unmount mandatory.

## Accessibility
TODO Phase 10. Targets:
- WCAG AA color contrast.
- All actions reachable via keyboard.
- Screen reader announces guess feedback (color → text via `aria-live`).
- `prefers-reduced-motion` disables non-essential tweens.

## Asset pipeline
TODO Phase 06. Sprites / fonts / audio in `web/static/`. SvelteKit hashes filenames at build time.
