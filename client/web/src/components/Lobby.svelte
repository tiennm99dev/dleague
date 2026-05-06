<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { auth } from '../auth/auth.svelte';
  import { WsClient, defaultWsUrl } from '../net/ws';
  import BetaBanner from './BetaBanner.svelte';
  import GameRunner from '../games/runner/GameRunner.svelte';

  const client = new WsClient(defaultWsUrl());
  let showGame = $state(false);

  onMount(() => {
    client.connect().catch(() => {
      // surfacing happens via client.lastError
    });
  });

  onDestroy(() => client.close());

  function startPlay() {
    showGame = true;
  }

  function exitGame() {
    showGame = false;
  }
</script>

<BetaBanner mode="topbar" />

<header class="topbar">
  <strong>dleague</strong>
  <span class="auth">
    Authenticated as {auth.user?.email ?? auth.user?.uid ?? '…'}
  </span>
  <span class="ws ws--{client.state}">
    WS: {client.state}
  </span>
  <button class="signout" onclick={() => auth.signOut()}>Sign out</button>
</header>

{#if client.lastError}
  <p class="error" role="alert">WS error: {client.lastError}</p>
{/if}

<main class="content">
  {#if !showGame}
    <p>Welcome back. Today's puzzle is waiting.</p>
    <button class="play" onclick={startPlay}>Play today's puzzle</button>
  {:else}
    <GameRunner variantKey="wordle" onExit={exitGame} />
  {/if}
</main>

<style>
  .topbar {
    display: flex;
    align-items: center;
    gap: 1rem;
    padding: 0.5rem 1rem;
    border-bottom: 1px solid #eee;
    font-family: system-ui, sans-serif;
  }
  .auth {
    color: #555;
    font-size: 0.9rem;
  }
  .ws {
    margin-left: auto;
    font-size: 0.8rem;
    padding: 0.15rem 0.5rem;
    border-radius: 3px;
    background: #eee;
  }
  .ws--connected {
    background: #d4f7d4;
    color: #135c13;
  }
  .ws--authenticating,
  .ws--connecting {
    background: #fff3cd;
    color: #856404;
  }
  .ws--closed {
    background: #fadbd8;
    color: #922b21;
  }
  .signout {
    background: transparent;
    border: 1px solid #ccc;
    border-radius: 4px;
    padding: 0.25rem 0.6rem;
    cursor: pointer;
  }
  .error {
    margin: 0.5rem 1rem;
    color: #c0392b;
    background: #fadbd8;
    padding: 0.5rem;
    border-radius: 4px;
  }
  .content {
    padding: 1rem;
    text-align: center;
    font-family: system-ui, sans-serif;
  }
  .play {
    margin-top: 1rem;
    padding: 0.75rem 1.5rem;
    font-size: 1rem;
    background: #2e7d32;
    color: white;
    border: 0;
    border-radius: 6px;
    cursor: pointer;
  }
</style>
