<script lang="ts">
  import { auth } from '../auth/auth.svelte';
  import SignIn from '../components/SignIn.svelte';
  import Lobby from '../components/Lobby.svelte';
</script>

{#if auth.loading}
  <div class="loading">Loading…</div>
{:else if !auth.user}
  <SignIn />
{:else}
  <Lobby />
{/if}

<style>
  .loading {
    margin: 5vh auto;
    text-align: center;
    font-family: system-ui, sans-serif;
    color: #777;
  }
</style>
