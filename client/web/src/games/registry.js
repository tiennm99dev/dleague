// Lazy-loaded variant registry. Each entry returns a Promise<GameVariant>
// so Vite splits the per-variant code into its own chunk — Phaser loads
// once, the variant chunk only when its game is played.

/** @typedef {import('./types').GameVariant} GameVariant */

/** @type {Map<string, () => Promise<GameVariant>>} */
export const variants = new Map([
  ['wordle', async () => (await import('./wordle')).default],
]);

/**
 * @param {string} key
 * @returns {Promise<GameVariant>}
 */
export async function loadVariant(key) {
  const loader = variants.get(key);
  if (!loader) throw new Error(`unknown game variant: ${key}`);
  return loader();
}
