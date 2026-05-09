<script lang="ts">
	// Solo daily and challenge-match Wordle play route.
	// Query params:
	//   ?match=<matchID>  — present when playing a challenge (challengee flow)
	//   ?seed=<int64>     — seed override for challenge play (ignored for solo)
	// On game terminal:
	//   - challenge mode: sends ATTEMPT_SUBMIT with full guesses + time_ms + won
	//   - solo mode: offers "Challenge a friend" (sends CHALLENGE_CREATE to get share token)
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { create, toBinary, fromBinary } from '@bufbuild/protobuf';
	import { MessageType } from '$lib/pb/dleague/v1/envelope_pb';
	import {
		WordleMoveSchema,
		WordleStateSchema,
		Color as ProtoColor
	} from '$lib/pb/dleague/v1/wordle_pb';
	import type { WordleState as ProtoWordleState } from '$lib/pb/dleague/v1/wordle_pb';
	import {
		AttemptSubmitSchema,
		AttemptSubmitAckSchema,
		ChallengeCreateSchema,
		ChallengeCreateAckSchema
	} from '$lib/pb/dleague/v1/match_pb';
	import { authUser } from '$lib/auth-store';
	import { sendRequest } from '$lib/ws';
	import { eventBus } from '$lib/phaser/event-bus';
	import Board from '$lib/components/board.svelte';
	import Keyboard from '$lib/components/keyboard.svelte';
	import ResultsScreen from '$lib/components/results-screen.svelte';
	import type { Color } from '$lib/game/wordle/colors';
	import Phaser from 'phaser';
	import { WordleScene } from '$lib/phaser/scenes/wordle-scene';

	// ── Query param context ───────────────────────────────────────────────────

	const matchId: string = $derived($page.url.searchParams.get('match') ?? '');
	const isChallengeMode: boolean = $derived(!!matchId);

	// ── Game state ────────────────────────────────────────────────────────────

	let guesses: string[] = $state([]);
	let hints: Color[][] = $state([]);
	let attemptsRemaining = $state(6);
	let won = $state(false);
	let lost = $state(false);
	let solution = $state('');
	let currentInput = $state('');
	let errorMsg = $state('');
	let submitting = $state(false);

	// Challenge / share state
	let attemptSubmitting = $state(false);
	let creating = $state(false); // guard against double-click on Challenge a friend
	let winnerUid = $state('');
	let shareToken = $state(''); // set after CHALLENGE_CREATE

	// Track game start time for time_ms calculation.
	let gameStartMs = 0;

	// ── Proto helpers ─────────────────────────────────────────────────────────

	function protoColorToClient(c: ProtoColor): Color {
		switch (c) {
			case ProtoColor.GREEN:
				return 'green';
			case ProtoColor.YELLOW:
				return 'yellow';
			default:
				return 'gray';
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

		if (row >= 0 && hints[row]) {
			eventBus.emit('wordle:flip-row', { row, colors: hints[row] });
		}

		// On terminal state, send async PvP submission if in challenge mode.
		if ((s.won || s.lost) && isChallengeMode) {
			void submitAttempt(s);
		}
	}

	// ── Async PvP: submit attempt ─────────────────────────────────────────────

	async function submitAttempt(s: ProtoWordleState): Promise<void> {
		if (!matchId) return;
		attemptSubmitting = true;
		const timeMs = Math.floor(Date.now() - gameStartMs);
		try {
			const msg = create(AttemptSubmitSchema, {
				matchId,
				guesses: s.guesses,
				timeMs,
				won: s.won
			});
			const respBytes = await sendRequest(
				MessageType.ATTEMPT_SUBMIT,
				toBinary(AttemptSubmitSchema, msg),
				10_000
			);
			const ack = fromBinary(AttemptSubmitAckSchema, respBytes);
			winnerUid = ack.winnerUid;
		} catch (err) {
			console.error('submitAttempt failed:', err);
		} finally {
			attemptSubmitting = false;
		}
	}

	// ── Solo daily: create challenge ──────────────────────────────────────────

	async function createChallenge(): Promise<void> {
		if (creating) return;
		creating = true;
		try {
			const msg = create(ChallengeCreateSchema, { gameId: 'wordle' });
			const respBytes = await sendRequest(
				MessageType.CHALLENGE_CREATE,
				toBinary(ChallengeCreateSchema, msg),
				10_000
			);
			const ack = fromBinary(ChallengeCreateAckSchema, respBytes);
			shareToken = ack.shareToken;
		} catch (err) {
			errorMsg =
				err instanceof Error ? err.message : 'Failed to create challenge';
			setTimeout(() => (errorMsg = ''), 3000);
		} finally {
			creating = false;
		}
	}

	// ── Keyboard / input ──────────────────────────────────────────────────────

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
		if (
			e.key === 'Enter' ||
			e.key === 'Backspace' ||
			(e.key.length === 1 && /[a-zA-Z]/.test(e.key))
		) {
			handleKey(e.key);
		}
	}

	async function submitGuess(): Promise<void> {
		if (currentInput.length !== 5) {
			errorMsg = 'Word must be 5 letters';
			setTimeout(() => (errorMsg = ''), 1500);
			return;
		}

		// Guard set synchronously before any await to prevent double-submit.
		if (submitting) return;
		submitting = true;
		errorMsg = '';

		try {
			const move = create(WordleMoveSchema, { guess: currentInput });
			const respBytes = await sendRequest(
				MessageType.WORDLE_MOVE,
				toBinary(WordleMoveSchema, move)
			);
			applyServerState(fromBinary(WordleStateSchema, respBytes));
		} catch (err) {
			errorMsg = err instanceof Error ? err.message : 'Server error';
			submitting = false;
		}
	}

	// ── Phaser ────────────────────────────────────────────────────────────────

	let phaserContainer: HTMLDivElement;
	let phaserGame: Phaser.Game | null = null;

	function initPhaser(): void {
		phaserGame = new Phaser.Game({
			type: Phaser.AUTO,
			width: 358,
			height: 408,
			backgroundColor: 'transparent',
			transparent: true,
			parent: phaserContainer,
			scene: [WordleScene],
			scale: { mode: Phaser.Scale.NONE }
		});
	}

	// ── Lifecycle ─────────────────────────────────────────────────────────────

	onMount(() => {
		window.addEventListener('keydown', handlePhysicalKey);
		initPhaser();
		gameStartMs = Date.now();
	});

	onDestroy(() => {
		window.removeEventListener('keydown', handlePhysicalKey);
		phaserGame?.destroy(true);
	});
</script>

<svelte:head>
	<title>Dleague — {isChallengeMode ? 'Challenge' : 'Daily Wordle'}</title>
</svelte:head>

<main class="play-root">
	<header class="play-header">
		<button class="back-btn" onclick={() => goto('/')}>← Back</button>
		<h1>{isChallengeMode ? 'Challenge' : 'Daily Wordle'}</h1>
		<span class="attempts">Attempts: {6 - attemptsRemaining}/6</span>
	</header>

	{#if errorMsg}
		<div class="toast" role="alert">{errorMsg}</div>
	{/if}

	<div class="board-wrapper">
		<Board {guesses} {hints} {currentInput} />
		<div class="phaser-overlay" bind:this={phaserContainer}></div>
	</div>

	{#if won || lost}
		<ResultsScreen
			{won}
			{solution}
			matchId={isChallengeMode ? matchId : undefined}
			winnerUid={winnerUid || undefined}
			currentUid={$authUser?.uid}
			shareToken={shareToken || undefined}
			onCreateChallenge={!isChallengeMode ? createChallenge : undefined}
			submitting={attemptSubmitting || creating}
		/>
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
		from {
			opacity: 0;
			transform: translateY(-4px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}

	.board-wrapper {
		position: relative;
	}

	.phaser-overlay {
		position: absolute;
		top: 0;
		left: 0;
		pointer-events: none;
	}

	.phaser-overlay :global(canvas) {
		display: block;
	}
</style>
