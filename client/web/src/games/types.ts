// GameVariant is the contract for a pluggable -dle game. Each variant
// supplies a Phaser scene class (canvas-side rendering + game logic) plus a
// Svelte component that renders the HUD overlay (attempt counter, win/lose
// modal, etc.). Auth and networking are owned by the runner — variants stay
// pure game.

import type { Scene } from 'phaser';
import type { Component } from 'svelte';

export interface GameVariantMeta {
  title: string;
  difficulty: 'easy' | 'medium' | 'hard';
  tagline: string;
}

export interface GameVariant {
  key: string;
  Scene: new (...args: any[]) => Scene;
  Hud: Component<any>;
  meta: GameVariantMeta;
}
