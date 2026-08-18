<script>
  // Sticky beta-stage notice. `mode` ∈ "full" (signin page, always visible)
  // or "topbar" (lobby, dismissable per session via sessionStorage).
  /** @type {{ mode?: 'full' | 'topbar' }} */
  let { mode = 'full' } = $props();

  const sessionKey = 'dleague.beta.dismissed';
  let dismissed = $state(
    typeof window !== 'undefined' && window.sessionStorage.getItem(sessionKey) === '1'
  );

  function dismiss() {
    dismissed = true;
    if (typeof window !== 'undefined') {
      window.sessionStorage.setItem(sessionKey, '1');
    }
  }
</script>

{#if mode === 'full' || !dismissed}
  <div class="banner banner--{mode}" role="status">
    <span class="badge">BETA</span>
    <span class="copy">
      Your data may reset before public release.
      Early adopters earn rewards as thanks for joining now.
    </span>
    {#if mode === 'topbar'}
      <button class="dismiss" onclick={dismiss} aria-label="Dismiss banner">×</button>
    {/if}
  </div>
{/if}

<style>
  .banner {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.5rem 1rem;
    background: #fff5d6;
    color: #6b4500;
    border: 1px solid #f0c674;
    border-radius: 6px;
    font-size: 0.9rem;
    line-height: 1.4;
  }
  .banner--full {
    margin: 1rem 0;
  }
  .banner--topbar {
    border-radius: 0;
    border-left: none;
    border-right: none;
  }
  .badge {
    background: #b97a00;
    color: white;
    padding: 0.1rem 0.5rem;
    border-radius: 3px;
    font-weight: bold;
    font-size: 0.75rem;
    letter-spacing: 0.5px;
  }
  .copy {
    flex: 1;
  }
  .dismiss {
    background: transparent;
    border: 0;
    color: inherit;
    cursor: pointer;
    font-size: 1.25rem;
    line-height: 1;
    padding: 0 0.25rem;
  }
</style>
