<script lang="ts">
	interface Props {
		inline?: boolean;
	}
	const { inline = false }: Props = $props();
</script>

{#if inline}
	<small class="anon-warning">
		Anonymous play — your daily leaderboard score won't be saved. Sign in to compete.
	</small>
{:else}
	<div class="anon-banner" role="status">
		Anonymous mode — sign in to save your daily leaderboard score.
	</div>
{/if}

<style>
	.anon-warning {
		display: block;
		color: #b8860b;
		background: #2a200a;
		border: 1px solid #b8860b;
		border-radius: 4px;
		padding: 0.3rem 0.6rem;
		font-size: 0.82rem;
		max-width: 320px;
		text-align: center;
	}

	.anon-banner {
		position: fixed;
		top: 2.5rem;
		left: 50%;
		transform: translateX(-50%);
		color: #b8860b;
		background: #2a200a;
		border: 1px solid #b8860b;
		border-radius: 4px;
		padding: 0.35rem 1rem;
		font-family: monospace;
		font-size: 0.82rem;
		z-index: 999;
		white-space: nowrap;
	}
</style>
