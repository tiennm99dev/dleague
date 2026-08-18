// GameVariant is the contract for a pluggable -dle game. Each variant
// supplies a Phaser scene class (canvas-side rendering + game logic) plus a
// Svelte component that renders the HUD overlay (attempt counter, win/lose
// modal, etc.). Auth and networking are owned by the runner — variants stay
// pure game.

/**
 * @typedef {Object} GameVariantMeta
 * @property {string} title
 * @property {'easy' | 'medium' | 'hard'} difficulty
 * @property {string} tagline
 */

/**
 * @typedef {Object} GameVariant
 * @property {string} key
 * @property {new (...args: any[]) => import('phaser').Scene} Scene
 * @property {import('svelte').Component<any>} Hud
 * @property {GameVariantMeta} meta
 */

export {};
