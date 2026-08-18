<script>
  // Generic game shell. Loads the requested variant via dynamic import,
  // fetches today's puzzle (auth'd → includes word for client-side per-guess
  // feedback), fetches resume state, mounts the Phaser scene + Svelte HUD,
  // POSTs the final attempt on completion.

  import { onMount, onDestroy } from 'svelte';
  import { auth } from '../../auth/auth.svelte';
  import { loadVariant } from '../registry';
  import { onAttemptComplete } from './eventbus-helpers';

  /** @typedef {import('../types').GameVariant} GameVariant */
  /** @typedef {import('phaser').Game} Game */
  /** @typedef {import('./eventbus-helpers').AttemptCompletePayload} AttemptCompletePayload */

  /** @type {{ variantKey: string, onExit?: () => void }} */
  let { variantKey, onExit = () => {} } = $props();

  /** @typedef {'loading' | 'ready' | 'finished' | 'error'} Stage */
  let stage = $state(/** @type {Stage} */ ('loading'));
  let errorMsg = $state(/** @type {string | null} */ (null));
  let variant = $state(/** @type {GameVariant | null} */ (null));
  let puzzleDate = $state(/** @type {string} */ (''));
  /** @type {Game | null} */
  let game = null;
  /** @type {(() => void) | null} */
  let unsubAttempt = null;

  /**
   * @typedef {Object} PuzzleResponse
   * @property {string} date
   * @property {string} word
   * @property {string} [hint]
   * @property {number} [difficulty]
   */

  /**
   * @param {string} path
   * @param {RequestInit} [init]
   * @returns {Promise<Response>}
   */
  async function authedFetch(path, init) {
    const token = await auth.getIdToken(false);
    if (!token) throw new Error('not signed in');
    return fetch(path, {
      ...init,
      headers: { ...(init?.headers ?? {}), Authorization: `Bearer ${token}` },
    });
  }

  /** @returns {Promise<PuzzleResponse>} */
  async function fetchPuzzle() {
    const res = await authedFetch('/api/v1/puzzles/me/today');
    if (!res.ok) throw new Error(`puzzle fetch failed: ${res.status}`);
    return res.json();
  }

  /**
   * @param {string} date
   * @returns {Promise<string[] | null>}
   */
  async function fetchResume(date) {
    const res = await authedFetch(`/api/v1/attempts/me/${date}`);
    if (res.status === 404) return null;
    if (!res.ok) throw new Error(`resume fetch failed: ${res.status}`);
    const a = await res.json();
    return Array.isArray(a.guesses) ? a.guesses : null;
  }

  /**
   * @param {string} date
   * @param {string[]} guesses
   * @returns {Promise<void>}
   */
  async function postAttempt(date, guesses) {
    const res = await authedFetch('/api/v1/attempts', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ date, guesses }),
    });
    if (!res.ok) throw new Error(`attempt post failed: ${res.status}`);
  }

  /** @param {string} parentId */
  async function startGame(parentId) {
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
      /** @type {Game} */ (game).scene.start('Wordle', initData);
    });
    if (game.isBooted) game.scene.start('Wordle', initData);
    stage = 'ready';
  }

  /** @param {AttemptCompletePayload} p */
  async function handleAttemptComplete(p) {
    try {
      await postAttempt(puzzleDate, p.guesses);
    } catch (e) {
      errorMsg = /** @type {Error} */ (e).message;
    }
    stage = 'finished';
  }

  onMount(async () => {
    unsubAttempt = onAttemptComplete(handleAttemptComplete);
    try {
      await startGame('game-container');
    } catch (e) {
      errorMsg = /** @type {Error} */ (e).message;
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
