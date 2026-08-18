<script>
  import { auth } from '../auth/auth.svelte';
  import BetaBanner from './BetaBanner.svelte';

  let email = $state('');
  let password = $state('');

  async function withGoogle() {
    await auth.signInWithGoogle();
  }
  async function withEmail() {
    if (!email || !password) return;
    await auth.signInWithEmail(email, password);
  }
  async function asGuest() {
    await auth.signInAnonymously();
  }
</script>

<div class="page">
  <h1>dleague</h1>
  <p class="tagline">League of -dle games — race opponents in real time.</p>

  <BetaBanner mode="full" />

  {#if auth.error}
    <p class="error" role="alert">{auth.error}</p>
  {/if}

  <button class="provider provider--google" onclick={withGoogle} disabled={auth.loading}>
    Continue with Google
  </button>
  <button class="provider provider--anon" onclick={asGuest} disabled={auth.loading}>
    Play as guest (anonymous)
  </button>

  <form
    class="email"
    onsubmit={(/** @type {SubmitEvent} */ e) => {
      e.preventDefault();
      withEmail();
    }}
  >
    <input type="email" placeholder="email" bind:value={email} autocomplete="email" />
    <input
      type="password"
      placeholder="password"
      bind:value={password}
      autocomplete="current-password"
    />
    <button type="submit" disabled={auth.loading}>Sign in with email</button>
  </form>

  <p class="cta">
    Join the beta — data may reset; early adopters get rewards.
  </p>
</div>

<style>
  .page {
    max-width: 420px;
    margin: 5vh auto;
    padding: 1.5rem;
    font-family: system-ui, sans-serif;
  }
  h1 {
    font-size: 2rem;
    margin: 0 0 0.25rem;
  }
  .tagline {
    margin: 0 0 1rem;
    color: #555;
  }
  .provider {
    display: block;
    width: 100%;
    padding: 0.75rem;
    margin: 0.5rem 0;
    border: 1px solid #ccc;
    border-radius: 6px;
    background: white;
    cursor: pointer;
    font-size: 1rem;
  }
  .provider--google {
    background: #4285f4;
    color: white;
    border-color: #3367d6;
  }
  .provider--anon {
    background: #f5f5f5;
  }
  .email {
    margin: 1rem 0;
    display: grid;
    gap: 0.5rem;
  }
  .email input,
  .email button {
    padding: 0.6rem;
    font-size: 1rem;
  }
  .error {
    color: #c0392b;
    background: #fadbd8;
    padding: 0.5rem;
    border-radius: 4px;
  }
  .cta {
    color: #777;
    font-size: 0.85rem;
    margin-top: 1rem;
  }
</style>
