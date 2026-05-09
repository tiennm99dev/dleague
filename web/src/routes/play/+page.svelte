<script lang="ts">
	// Solo daily Wordle play route.
	// Connects to the WS server, sends GAME_MOVE on each submitted guess,
	// updates state from GAME_STATE responses, and emits flip-row events for
	// the Phaser overlay to animate.
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { create, toBinary, fromBinary } from '@bufbuild/protobuf';
	import { MessageType } from '$lib/pb/dleague/v1/envelope_pb';
	import {
		WordleMoveSchema,
		WordleStateSchema,
		Color as ProtoColor
	} from '$lib/pb/dleague/v1/wordle_pb';
	import type { WordleState as ProtoWordleState } from '$lib/pb/dleague/v1/wordle_pb';
	import { authUser, idToken } from '$lib/auth-store';
	import { connect, disconnect, sendRequest, onMessage, removeHandler, connectionState } from '$lib/ws';
	import { eventBus } from '$lib/phaser/event-bus';
	import Board from '$lib/components/board.svelte';
	import Keyboard from '$lib/components/keyboard.svelte';
	import type { Color } from '$lib/game/wordle/colors';
	import Phaser from 'phaser';
	import { WordleScene } from '$lib/phaser/scenes/wordle-scene';

	// ── State ────────────────────────────────────────────────────────────────

	let guesses: string[] = $state([]);
	let hints: Color[][] = $state([]);
	let attemptsRemaining = $state(6);
	let won = $state(false);
	let lost = $state(false);
	let solution = $state('');
	let currentInput = $state('');
	let errorMsg = $state('');
	let submitting = $state(false);

	// ── Proto Color → client Color ────────────────────────────────────────────

	function protoColorToClient(c: ProtoColor): Color {
		switch (c) {
			case ProtoColor.GREEN:  return 'green';
			case ProtoColor.YELLOW: return 'yellow';
			default:                return 'gray';
		}
	}

	function applyServerState(s: ProtoWordleState): void {
		const row = s.guesses.length - 1;
		guesses = [...s.guesses];
		hints = s.hints.map((h) => h.colors.map(protoColorToClient));
		attemptsRemaining = s.attemptsRemaining;
		won = s.won;
		lost = s.lost;
		solution = s.solution ?? '';
		currentInput = '';
		submitting = false;

		// Trigger Phaser tile-flip for the just-submitted row.
		if (row >= 0 && hints[row]) {
			eventBus.emit('wordle:flip-row', { row, colors: hints[row] });
		}
	}

	// ── Keyboard / input handling ─────────────────────────────────────────────

	function handleKey(key: string): void {
		if (won || lost || submitting) return;
		if (key === 'Enter') {
			void submitGuess();
		} else if (key === 'Backspace') {
			currentInput = currentInput.slice(0, -1);
		} else if (/^[A-Za-z]$/.test(key) && currentInput.length < 5) {
			currentInput += key.toUpperCase();
		}
	}

	function handlePhysicalKey(e: KeyboardEvent): void {
		handleKey(e.key === 'Backspace' ? 'Backspace' : e.key.length === 1 ? e.key : e.key);
	}

	// ── Submit guess ──────────────────────────────────────────────────────────

	async function submitGuess(): Promise<void> {
		if (currentInput.length !== 5) {
			errorMsg = 'Word must be 5 letters';
			setTimeout(() => (errorMsg = ''), 1500);
			return;
		}
		if ($connectionState !== 'connected') {
			errorMsg = 'Not connected — please wait';
			setTimeout(() => (errorMsg = ''), 2000);
			return;
		}

		submitting = true;
		errorMsg = '';

		try {
			const move = create(WordleMoveSchema, { guess: currentInput });
			const payload = toBinary(WordleMoveSchema, move);
			const respBytes = await sendRequest(MessageType.GAME_MOVE, payload);
			const state = fromBinary(WordleStateSchema, respBytes);
			applyServerState(state);
		} catch (err) {
			errorMsg = err instanceof Error ? err.message : 'Server error';
			submitting = false;
		}
	}

	// ── Phaser setup ──────────────────────────────────────────────────────────

	let phaserContainer: HTMLDivElement;
	let phaserGame: Phaser.Game | null = null;

	function initPhaser(): void {
		phaserGame = new Phaser.Game({
			type: Phaser.AUTO,
			width: 358,  // 5 tiles × (56+6) − 6 + 24 padding
			height: 408, // 6 rows × (56+6) − 6 + 24 padding
			backgroundColor: 'transparent',
			transparent: true,
			parent: phaserContainer,
			scene: [WordleScene],
			scale: { mode: Phaser.Scale.NONE }
		});
	}

	// ── Lifecycle ─────────────────────────────────────────────────────────────

	onMount(async () => {
		window.addEventListener('keydown', handlePhysicalKey);
		initPhaser();

		// Connect WS using current Firebase token.
		try {
			const token = await idToken();
			connect(token);
		} catch {
			// Anonymous / unauthenticated — connect without token for dev.
			connect('');
		}

		// Register GAME_STATE push handler (for future server-push; requests
		// use the sendRequest Promise path, but register defensively).
		onMessage(MessageType.GAME_STATE, (payload) => {
			try {
				const state = fromBinary(WordleStateSchema, payload);
				applyServerState(state);
			} catch {
				// ignore malformed push
			}
		});
	});

	onDestroy(() => {
		window.removeEventListener('keydown', handlePhysicalKey);
		removeHandler(MessageType.GAME_STATE);
		phaserGame?.destroy(true);
		disconnect();
	});
</script>

<svelte:head>
	<title>Dleague — Daily Wordle</title>
</svelte:head>

<main class="play-root">
	<header class="play-header">
		<button class="back-btn" onclick={() => goto('/')}>← Back</button>
		<h1>Daily Wordle</h1>
		<span class="attempts">Attempts: {6 - attemptsRemaining}/6</span>
	</header>

	{#if errorMsg}
		<div class="toast" role="alert">{errorMsg}</div>
	{/if}

	<!-- Board + Phaser overlay wrapper -->
	<div class="board-wrapper">
		<Board {guesses} {hints} {currentInput} />
		<!-- Phaser canvas overlays the board for tile-flip animations -->
		<div class="phaser-overlay" bind:this={phaserContainer}></div>
	</div>

	{#if won}
		<div class="result result--win">
			<p>You won! 🎉</p>
			<p>Solution: <strong>{solution}</strong></p>
			<button onclick={() => goto('/')}>Back to home</button>
		</div>
	{:else if lost}
		<div class="result result--loss">
			<p>Better luck tomorrow!</p>
			<p>Solution: <strong>{solution}</strong></p>
			<button onclick={() => goto('/')}>Back to home</button>
		</div>
	{:else}
		<Keyboard {hints} {guesses} onkey={handleKey} />
	{/if}
</main>

<style>
	.play-root {
		display: flex;
		flex-direction: column;
		align-items: center;
		min-height: 100vh;
		background: #121213;
		color: #ffffff;
		font-family: monospace;
		gap: 8px;
		padding-bottom: 24px;
	}

	.play-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		width: 100%;
		max-width: 500px;
		padding: 12px 16px;
		border-bottom: 1px solid #3a3a3c;
	}

	.play-header h1 {
		font-size: 1.4rem;
		letter-spacing: 0.1em;
		margin: 0;
	}

	.back-btn {
		background: none;
		border: none;
		color: #ffffff;
		cursor: pointer;
		font-family: monospace;
		font-size: 0.9rem;
	}

	.attempts {
		font-size: 0.85rem;
		color: #818384;
	}

	.toast {
		background: #b00020;
		color: #ffffff;
		padding: 8px 16px;
		border-radius: 4px;
		font-size: 0.9rem;
		animation: fadeIn 0.15s ease;
	}

	@keyframes fadeIn {
		from { opacity: 0; transform: translateY(-4px); }
		to   { opacity: 1; transform: translateY(0); }
	}

	.board-wrapper {
		position: relative;
	}

	.phaser-overlay {
		position: absolute;
		top: 0;
		left: 0;
		pointer-events: none; /* clicks pass through to Svelte board */
	}

	/* Ensure Phaser canvas is transparent overlay */
	.phaser-overlay :global(canvas) {
		display: block;
	}

	.result {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 8px;
		padding: 24px;
		border-radius: 8px;
		margin-top: 16px;
	}

	.result--win  { background: #1a3a1a; }
	.result--loss { background: #2a1a1a; }

	.result p { margin: 0; }

	.result button {
		margin-top: 12px;
		padding: 8px 20px;
		background: #538d4e;
		border: none;
		border-radius: 4px;
		color: #ffffff;
		font-family: monospace;
		cursor: pointer;
	}
</style>
