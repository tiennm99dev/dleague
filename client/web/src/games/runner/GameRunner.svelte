<script lang="ts">
  // Generic game shell. Loads the requested variant via dynamic import,
  // fetches today's puzzle (auth'd → includes word for client-side per-guess
  // feedback), fetches resume state, mounts the Phaser scene + Svelte HUD,
  // POSTs the final attempt on completion.

  import { onMount, onDestroy } from 'svelte';
  import { auth } from '../../auth/auth.svelte';
  import { loadVariant } from '../registry';
  import type { GameVariant } from '../types';
  import type { Game } from 'phaser';
  import { onAttemptComplete, type AttemptCompletePayload } from './eventbus-helpers';

  let { variantKey, onExit = () => {} } = $props();

  type Stage = 'loading' | 'ready' | 'finished' | 'error';
  let stage = $state<Stage>('loading');
  let errorMsg = $state<string | null>(null);
  let variant = $state<GameVariant | null>(null);
  let puzzleDate = $state<string>('');
  let game: Game | null = null;
  let unsubAttempt: (() => void) | null = null;

  interface PuzzleResponse {
    date: string;
    word: string;
    hint?: string;
    difficulty?: number;
  }

  async function authedFetch(path: string, init?: RequestInit): Promise<Response> {
    const token = await auth.getIdToken(false);
    if (!token) throw new Error('not signed in');
    return fetch(path, {
      ...init,
      headers: { ...(init?.headers ?? {}), Authorization: `Bearer ${token}` },
    });
  }

  async function fetchPuzzle(): Promise<PuzzleResponse> {
    const res = await authedFetch('/api/v1/puzzles/me/today');
    if (!res.ok) throw new Error(`puzzle fetch failed: ${res.status}`);
    return res.json();
  }

  async function fetchResume(date: string): Promise<string[] | null> {
    const res = await authedFetch(`/api/v1/attempts/me/${date}`);
    if (res.status === 404) return null;
    if (!res.ok) throw new Error(`resume fetch failed: ${res.status}`);
    const a = await res.json();
    return Array.isArray(a.guesses) ? a.guesses : null;
  }

  async function postAttempt(date: string, guesses: string[]): Promise<void> {
    const res = await authedFetch('/api/v1/attempts', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ date, guesses }),
    });
    if (!res.ok) throw new Error(`attempt post failed: ${res.status}`);
  }

  async function startGame(parentId: string) {
    const v = await loadVariant(variantKey);
    variant = v;
    const puzzle = await fetchPuzzle();
    puzzleDate = puzzle.date;
    const resumeGuesses = await fetchResume(puzzle.date);
    const phaser = await import('phaser');
    game = new phaser.Game({
      type: phaser.AUTO,
      width: 600,
      height: 720,
      parent: parentId,
      backgroundColor: '#ffffff',
      scene: [v.Scene],
    });
    const initData = {
      solution: puzzle.word,
      length: puzzle.word.length,
      resumeGuesses: resumeGuesses ?? undefined,
    };
    // Wait until Phaser is up before starting the scene.
    game.events.once('ready', () => {
      game!.scene.start('Wordle', initData);
    });
    if (game.isBooted) game.scene.start('Wordle', initData);
    stage = 'ready';
  }

  async function handleAttemptComplete(p: AttemptCompletePayload) {
    try {
      await postAttempt(puzzleDate, p.guesses);
    } catch (e) {
      errorMsg = (e as Error).message;
    }
    stage = 'finished';
  }

  onMount(async () => {
    unsubAttempt = onAttemptComplete(handleAttemptComplete);
    try {
      await startGame('game-container');
    } catch (e) {
      errorMsg = (e as Error).message;
      stage = 'error';
    }
  });

  onDestroy(() => {
    unsubAttempt?.();
    game?.destroy(true);
    game = null;
  });
</script>

<div class="runner">
  {#if stage === 'loading'}
    <p class="status">Loading {variantKey}…</p>
  {:else if stage === 'error'}
    <p class="error">Failed to start: {errorMsg}</p>
    <button onclick={onExit}>Back</button>
  {/if}

  <div class="canvas-wrap">
    <div id="game-container"></div>
    {#if variant}
      {@const Hud = variant.Hud}
      <Hud date={puzzleDate} onDone={() => {}} />
    {/if}
  </div>

  <div class="footer">
    <button onclick={onExit}>Back to lobby</button>
  </div>
</div>

<style>
  .runner {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.5rem;
  }
  .canvas-wrap {
    position: relative;
    width: 600px;
    max-width: 100%;
  }
  .status {
    color: #555;
  }
  .error {
    color: #c0392b;
  }
  .footer {
    margin-top: 0.5rem;
  }
  .footer button {
    background: transparent;
    border: 1px solid #ccc;
    padding: 0.4rem 0.8rem;
    border-radius: 4px;
    cursor: pointer;
  }
</style>
