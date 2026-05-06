import type { GameVariant } from '../types';
import { WordleScene } from './WordleScene';
import WordleHud from './WordleHud.svelte';

const variant: GameVariant = {
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
