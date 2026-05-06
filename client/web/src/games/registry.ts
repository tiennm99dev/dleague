// Lazy-loaded variant registry. Each entry returns a Promise<GameVariant>
// so Vite splits the per-variant code into its own chunk — Phaser loads
// once, the variant chunk only when its game is played.

import type { GameVariant } from './types';

export const variants = new Map<string, () => Promise<GameVariant>>([
  ['wordle', async () => (await import('./wordle')).default],
]);

export async function loadVariant(key: string): Promise<GameVariant> {
  const loader = variants.get(key);
  if (!loader) throw new Error(`unknown game variant: ${key}`);
  return loader();
}
