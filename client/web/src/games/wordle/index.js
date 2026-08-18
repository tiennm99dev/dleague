import { WordleScene } from './WordleScene';
import WordleHud from './WordleHud.svelte';

/** @typedef {import('../types').GameVariant} GameVariant */

/** @type {GameVariant} */
const variant = {
  key: 'wordle',
  Scene: WordleScene,
  Hud: WordleHud,
  meta: {
    title: 'Wordle',
    difficulty: 'medium',
    tagline: 'Guess the daily word in six tries.',
  },
};

export default variant;
