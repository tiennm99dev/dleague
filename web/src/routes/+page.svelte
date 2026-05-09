<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import PhaserGame from '$lib/phaser/phaser-game.svelte';
	import { eventBus } from '$lib/phaser/event-bus';

	onMount(() => {
		// When the title scene signals start, navigate to the (future) play route.
		eventBus.on('title:start', handleStart);
		return () => eventBus.off('title:start', handleStart);
	});

	function handleStart(): void {
		goto('/play');
	}
</script>

<main>
	<PhaserGame />
</main>

<style>
	main {
		display: flex;
		justify-content: center;
		align-items: center;
		min-height: 100vh;
		background: #1a1a2e;
	}
</style>
