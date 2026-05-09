<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import PhaserGame from '$lib/phaser/phaser-game.svelte';
	import { eventBus } from '$lib/phaser/event-bus';

	onMount(() => {
		// When the title scene signals start, navigate to the solo play route.
		eventBus.on('title:start', handleStart);
		return () => eventBus.off('title:start', handleStart);
	});

	function handleStart(): void {
		goto('/play');
	}

	function handleQuickMatch(): void {
		goto('/quick-match');
	}
</script>

<main>
	<PhaserGame />
	<div class="cta-row">
		<button class="btn-quick-match" onclick={handleQuickMatch}>
			Quick Match
		</button>
	</div>
</main>

<style>
	main {
		display: flex;
		flex-direction: column;
		justify-content: center;
		align-items: center;
		min-height: 100vh;
		background: #1a1a2e;
	}

	.cta-row {
		margin-top: 16px;
	}

	.btn-quick-match {
		padding: 12px 32px;
		background: #538d4e;
		color: #fff;
		border: none;
		border-radius: 6px;
		font-size: 1rem;
		font-weight: 600;
		cursor: pointer;
		letter-spacing: 0.05em;
		transition: background 0.2s;
	}

	.btn-quick-match:hover {
		background: #6aaf64;
	}
</style>
