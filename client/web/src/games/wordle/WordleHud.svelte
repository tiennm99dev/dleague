<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { onAttemptComplete, onGuessSubmitted, type AttemptCompletePayload } from '../runner/eventbus-helpers';
  import { MAX_GUESSES } from './scoring';

  let { date = '', onDone = (_p: AttemptCompletePayload) => {} } = $props();

  let guessesUsed = $state(0);
  let result = $state<AttemptCompletePayload | null>(null);

  let unsubGuess: (() => void) | null = null;
  let unsubComplete: (() => void) | null = null;

  onMount(() => {
    unsubGuess = onGuessSubmitted(() => {
      guessesUsed += 1;
    });
    unsubComplete = onAttemptComplete((p) => {
      result = p;
      onDone(p);
    });
  });

  onDestroy(() => {
    unsubGuess?.();
    unsubComplete?.();
  });

  function shareText(): string {
    if (!result) return '';
    const grid = result.evaluations
      .map((row) => row.map((e) => (e === 'hit' ? '🟩' : e === 'present' ? '🟨' : '⬛')).join(''))
      .join('\n');
    const tries = result.status === 'won' ? `${result.guesses.length}/${MAX_GUESSES}` : `X/${MAX_GUESSES}`;
    return `dleague Wordle ${date} ${tries}\n\n${grid}`;
  }

  function copyShare() {
    void navigator.clipboard?.writeText(shareText());
  }
</script>

<div class="hud">
  <div class="meta">
    <span>Guesses: <strong>{guessesUsed}</strong> / {MAX_GUESSES}</span>
    <span class="date">{date}</span>
  </div>

  {#if result}
    <div class="modal" role="dialog" aria-label="Result">
      <h2>{result.status === 'won' ? 'You won!' : 'Out of guesses'}</h2>
      <pre class="share-grid">{shareText()}</pre>
      <button onclick={copyShare}>Copy share text</button>
    </div>
  {/if}
</div>

<style>
  .hud {
    position: absolute;
    inset: 0;
    pointer-events: none;
    font-family: system-ui, sans-serif;
  }
  .meta {
    pointer-events: auto;
    display: flex;
    justify-content: space-between;
    padding: 0.5rem 1rem;
    color: #444;
    font-size: 0.9rem;
  }
  .date {
    color: #888;
    font-variant-numeric: tabular-nums;
  }
  .modal {
    pointer-events: auto;
    position: absolute;
    left: 50%;
    top: 50%;
    transform: translate(-50%, -50%);
    background: white;
    border: 1px solid #ddd;
    border-radius: 8px;
    padding: 1rem 1.5rem;
    box-shadow: 0 4px 24px rgba(0, 0, 0, 0.12);
    text-align: center;
    min-width: 220px;
  }
  .modal h2 {
    margin: 0 0 0.5rem 0;
  }
  .share-grid {
    font-family: ui-monospace, monospace;
    background: #f7f7f7;
    padding: 0.5rem;
    border-radius: 4px;
    text-align: left;
    margin: 0.5rem 0;
  }
  .modal button {
    margin-top: 0.5rem;
    padding: 0.4rem 1rem;
    background: #2e7d32;
    color: white;
    border: 0;
    border-radius: 4px;
    cursor: pointer;
  }
</style>
