<script lang="ts">
	// Svelte wrapper that owns the Phaser.Game lifecycle.
	// Creates the game on mount; destroys it on unmount to prevent memory leaks.
	// EventBus is the only communication channel between scenes and Svelte.
	import { onMount } from 'svelte';
	import Phaser from 'phaser';
	import { TitleScene } from './scenes/title-scene';

	let container: HTMLDivElement;

	onMount(() => {
		const game = new Phaser.Game({
			type: Phaser.AUTO,
			width: 800,
			height: 600,
			backgroundColor: '#1a1a2e',
			parent: container,
			scene: [TitleScene],
			scale: {
				mode: Phaser.Scale.FIT,
				autoCenter: Phaser.Scale.CENTER_BOTH
			},
			render: {
				pixelArt: false,
				antialias: true
			}
		});

		return () => {
			// Phaser.destroy(true) removes the canvas from the DOM and frees GL ctx.
			game.destroy(true);
		};
	});
</script>

<div bind:this={container} class="phaser-container"></div>

<style>
	.phaser-container {
		width: 100%;
		max-width: 800px;
		aspect-ratio: 4 / 3;
		position: relative;
	}

	/* Ensure the Phaser-generated canvas fills the container */
	.phaser-container :global(canvas) {
		display: block;
		width: 100% !important;
		height: 100% !important;
	}
</style>
