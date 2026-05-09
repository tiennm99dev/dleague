# Design Guidelines

UI/UX principles, palette, animation timing, accessibility, and mobile
breakpoints for the Dleague client (SvelteKit + Phaser).

## Palette — Wordle mode

| Token | Hex | Usage |
|-------|-----|-------|
| `--color-correct` | `#6aaa64` | Green: letter in correct position |
| `--color-present` | `#c9b458` | Yellow: letter in word, wrong position |
| `--color-absent` | `#787c7e` | Gray: letter not in word |
| `--color-empty` | `#121213` | Dark: unfilled tile background |
| `--color-tile-border` | `#3a3a3c` | Tile border (dark mode) |
| `--color-key-bg` | `#818384` | Keyboard key default |
| `--color-bg` | `#121213` | Page background |
| `--color-text` | `#ffffff` | Primary text |

Dark mode only for MVP. Light mode is v2.

## Typography

- Font family: `'Clear Sans', 'Helvetica Neue', Arial, sans-serif`
- Board tile: bold 2 rem, letter-spacing 0.1em, uppercase
- Keyboard key: bold 0.875 rem
- Headings: bold, all-caps

## Tile Animations

| Animation | Duration | Easing | Notes |
|-----------|----------|--------|-------|
| Tile flip (each half) | 150 ms | `Linear` | Y-axis scale 1→0→1; recolor at midpoint |
| Tile stagger | 100 ms per column | — | Left-to-right wave effect |
| Tile fade-out | 200 ms | `Linear` | After 300 ms hold; reveals DOM tile below |
| Invalid-word shake | 400 ms | `ease-in-out` | Horizontal shake on bad guess |

Total visible flip animation per row: `5 × 100 + 2 × 150 + 300 + 200 = 1,350 ms`

### Reduced-motion fallback

When `prefers-reduced-motion: reduce` is active:
- Skip tween phases; apply final color directly (`rect.setFillStyle(color)`)
- Skip shake animation; flash border red instead
- Client-side detection: `window.matchMedia('(prefers-reduced-motion: reduce)').matches`

```ts
// In wordle-scene.ts flipRow():
if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
  rect.setFillStyle(COLOR_MAP[colors[col]] ?? 0x3a3a3c);
  return; // skip tween
}
```

## Mobile Breakpoints

| Breakpoint | Width | Layout changes |
|------------|-------|----------------|
| Small | ≤ 375 px | Board tiles shrink to 48×48; keyboard keys 36 px wide |
| Medium | ≤ 768 px | Single-column layout; header collapses |
| Large | > 768 px | Side-by-side layout (board + opponent panel) |

Touch targets: minimum 44×44 px per WCAG 2.5.5.

## Accessibility

- **ARIA labels:** each board cell has `aria-label="row 1, col 3: A, correct"`.
  Updated via Svelte reactive statements after server response.
- **Color contrast:** all text on tile colors verified ≥ 4.5:1 (WCAG AA):
  - White `#ffffff` on `#6aaa64` green: 3.5:1 — use bold weight to compensate,
    or darken green to `#538d4e` (Phaser `COLOR_MAP` already uses the darker value).
  - White on `#c9b458` yellow: 3.2:1 — dark text `#1a1a1b` on yellow tiles instead.
  - White on `#787c7e` gray: 4.6:1 — passes AA.
- **Keyboard-only play:** `Enter` submits guess; `Backspace` deletes; letter keys
  type. No mouse required for the core game loop.
- **Focus management:** after sign-in, focus moves to the first board cell.
- **Screen readers:** `role="grid"` on the board; `role="gridcell"` on tiles;
  `aria-live="polite"` region announces guess result after each row flip.

## Component Size Limits

- Each Svelte component: < 200 LOC (enforced by code standards).
- Phaser scene: no inline game logic; scenes only animate — state lives in the
  server response and Svelte stores.

## Bundle Budget

- Target: < 400 KB gzip total across all JS chunks.
- Current measured: ~411 KB gzip (Phaser 3.88 full build adds ~341 KB).
- Mitigation (v2): Phaser custom build (audio/physics stripped) saves ~100 KB gzip.
- Monitor with `npm run build` output; each new dependency must be reviewed.
