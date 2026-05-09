<script lang="ts">
	import { onMount, onDestroy, untrack } from 'svelte';
	import Board from './board.svelte';
	import Keyboard from './keyboard.svelte';
	import OpponentPanel from './opponent-panel.svelte';
	import ResultsScreen from './results-screen.svelte';
	import {
		sendMatchMove,
		onMatchOpponentProgress,
		onMatchResolved,
		removeHandler,
		onMessage
	} from '$lib/ws';
	import { fromBinary } from '@bufbuild/protobuf';
	import { WordleStateSchema } from '$lib/pb/dleague/v1/wordle_pb';
	import type { WordleState, WordleHint } from '$lib/pb/dleague/v1/wordle_pb';
	import type { Color as ProtoColor } from '$lib/pb/dleague/v1/wordle_pb';
	import type { Color as GameColor } from '$lib/game/wordle/colors';
	import { MessageType } from '$lib/pb/dleague/v1/envelope_pb';
	import { authUser } from '$lib/auth-store';
	import { get } from 'svelte/store';

	interface Props {
		matchId: string;
		seed: bigint;
		opponentName: string;
		/** Initial own state (populated on MATCH_REJOIN_ACK). */
		initialState?: WordleState | null;
		/** Initial opponent hints (populated on MATCH_REJOIN_ACK). */
		initialOpponentHints?: WordleHint[];
	}

	let {
		matchId,
		seed,
		opponentName,
		initialState = null,
		initialOpponentHints = []
	}: Props = $props();

	// Own game state — use untrack() to read initial prop values without
	// establishing reactive dependencies in the $state() initialisers.
	let ownGuesses = $state<string[]>(untrack(() => initialState?.guesses ?? []));
	let ownHints = $state<WordleHint[]>(untrack(() => initialState?.hints ?? []));
	let ownWon = $state(untrack(() => initialState?.won ?? false));
	let ownLost = $state(untrack(() => initialState?.lost ?? false));
	let currentInput = $state('');

	// Opponent state (colors only — no letters).
	let opponentRows = $state<ProtoColor[][]>(
		untrack(() => initialOpponentHints.map((h) => h.colors))
	);

	// Convert proto hints (WordleHint[] with integer Color enum) to the string-based
	// Color[][] format that Board expects.
	function protoHintsToGameColors(hints: WordleHint[]): GameColor[][] {
		return hints.map((h) =>
			h.colors.map((c): GameColor => {
				switch (c) {
					case 3: return 'green';
					case 2: return 'yellow';
					case 1: return 'gray';
					default: return 'gray';
				}
			})
		);
	}

	let boardHints = $derived(protoHintsToGameColors(ownHints));

	// Match result.
	let resolved = $state(false);
	let winnerUid = $state('');
	let resolveReason = $state('');
	let resultReason = $state<'win' | 'loss' | 'tie' | 'opponent-left' | 'self-disconnect'>('loss');
	let matchSolution = $state('');
	// Tracks solution received from GAME_STATE pushes (populated when terminal).
	let lastSolution = $state('');

	function handleKeyPress(key: string): void {
		if (resolved || ownWon || ownLost) return;

		if (key === 'Enter') {
			if (currentInput.length === 5) {
				sendMatchMove(matchId, currentInput);
				currentInput = '';
			}
		} else if (key === 'Backspace') {
			currentInput = currentInput.slice(0, -1);
		} else if (key.length === 1 && /[a-zA-Z]/.test(key) && currentInput.length < 5) {
			currentInput += key.toUpperCase();
		}
	}

	onMount(() => {
		// Subscribe to server-pushed own state (GAME_STATE carries own WordleState).
		onMessage(MessageType.GAME_STATE, (payload) => {
			const state = fromBinary(WordleStateSchema, payload);
			ownGuesses = [...state.guesses];
			ownHints = [...state.hints];
			ownWon = state.won;
			ownLost = state.lost;
			// solution is populated by server only when game is terminal.
			if (state.solution) lastSolution = state.solution;
		});

		onMatchOpponentProgress((msg) => {
			if (msg.matchId !== matchId) return;
			// Grow opponent rows to the attempt number.
			while (opponentRows.length < msg.attemptNum - 1) {
				opponentRows.push([]);
			}
			opponentRows = [...opponentRows.slice(0, msg.attemptNum - 1), [...msg.colors]];
		});

		onMatchResolved((msg) => {
			if (msg.matchId !== matchId) return;
			resolved = true;
			winnerUid = msg.winnerUid;
			resolveReason = msg.reason;
			// Use solution captured from last GAME_STATE push (MatchResolved has no solution field).
			matchSolution = lastSolution;
			// Derive result reason driven by reason field first (I2 fix).
			const selfUid = get(authUser)?.uid ?? '';
			if (msg.reason === 'exhausted') {
				// Both players ran out of guesses — a loss for self.
				resultReason = 'loss';
			} else if (msg.reason === 'forfeit' || msg.reason === 'timeout') {
				resultReason = msg.winnerUid === selfUid ? 'opponent-left' : 'self-disconnect';
			} else if (msg.reason === 'solved') {
				resultReason = msg.winnerUid === selfUid ? 'win' : 'loss';
			} else if (!msg.winnerUid) {
				// Genuine tie path (rare; no known server case today).
				resultReason = 'tie';
			} else {
				resultReason = msg.winnerUid === selfUid ? 'win' : 'loss';
			}
			// Clear active match from sessionStorage.
			sessionStorage.removeItem('activeMatchID');
		});

		// Keyboard handler for physical keyboard.
		function onKeyDown(e: KeyboardEvent): void {
			handleKeyPress(e.key);
		}
		window.addEventListener('keydown', onKeyDown);
		return () => window.removeEventListener('keydown', onKeyDown);
	});

	onDestroy(() => {
		removeHandler(MessageType.GAME_STATE);
		removeHandler(MessageType.MATCH_OPPONENT_PROGRESS);
		removeHandler(MessageType.MATCH_RESOLVED);
	});

</script>

<div class="sync-scene">
	{#if resolved}
		<div class="result-overlay">
			<ResultsScreen
				won={resultReason === 'win'}
				solution={matchSolution}
				matchId={matchId}
				winnerUid={winnerUid || undefined}
				currentUid={get(authUser)?.uid}
				reason={resultReason}
			/>
		</div>
	{/if}

	<div class="two-pane">
		<!-- Left pane: own board -->
		<div class="pane pane-self">
			<h3>You</h3>
			<Board guesses={ownGuesses} hints={boardHints} currentInput={currentInput} />
			<Keyboard onkey={handleKeyPress} hints={boardHints} guesses={ownGuesses} />
		</div>

		<!-- Right pane: opponent colors only -->
		<div class="pane pane-opponent">
			<OpponentPanel rows={opponentRows} opponentName={opponentName} />
		</div>
	</div>
</div>

<style>
	.sync-scene {
		position: relative;
		min-height: 100vh;
		background: #121213;
		color: #fff;
		display: flex;
		flex-direction: column;
		align-items: center;
		padding: 16px;
	}

	.two-pane {
		display: flex;
		gap: 48px;
		align-items: flex-start;
		justify-content: center;
		flex-wrap: wrap;
	}

	.pane {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 12px;
	}

	.pane h3 {
		margin: 0;
		font-size: 0.85rem;
		text-transform: uppercase;
		letter-spacing: 0.1em;
		color: #aaa;
	}

	.result-overlay {
		position: absolute;
		inset: 0;
		background: rgba(0, 0, 0, 0.8);
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 16px;
		z-index: 10;
	}

</style>
