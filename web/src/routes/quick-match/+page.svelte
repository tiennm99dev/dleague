<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { sendQueueJoin, sendQueueLeave, onQueueMatched, removeHandler } from '$lib/ws';
	import { MessageType } from '$lib/pb/dleague/v1/envelope_pb';

	let searching = $state(true);
	let statusText = $state('Searching for an opponent…');

	onMount(() => {
		// Join the queue immediately on mount.
		sendQueueJoin('wordle');

		onQueueMatched((msg) => {
			searching = false;
			statusText = `Matched against ${msg.opponentDisplayName}! Starting…`;
			// Persist match details in sessionStorage for reconnect.
			sessionStorage.setItem('activeMatchID', msg.matchId);
			sessionStorage.setItem('activeSeed', msg.seed.toString());
			sessionStorage.setItem('activeOpponent', msg.opponentDisplayName);
			// Navigate to the sync game route.
			goto(`/sync?matchId=${encodeURIComponent(msg.matchId)}&seed=${msg.seed.toString()}&opponent=${encodeURIComponent(msg.opponentDisplayName)}`);
		});
	});

	onDestroy(() => {
		removeHandler(MessageType.QUEUE_MATCHED);
		if (searching) {
			sendQueueLeave();
		}
	});

	function cancel(): void {
		searching = false;
		sendQueueLeave();
		goto('/');
	}
</script>

<div class="quick-match-page">
	<div class="card">
		<h1>Quick Match</h1>
		<p class="status">{statusText}</p>
		{#if searching}
			<div class="spinner" aria-label="Searching"></div>
			<button class="btn-cancel" onclick={cancel}>Cancel</button>
		{/if}
	</div>
</div>

<style>
	.quick-match-page {
		min-height: 100vh;
		background: #121213;
		display: flex;
		align-items: center;
		justify-content: center;
	}

	.card {
		background: #1a1a2e;
		border-radius: 12px;
		padding: 40px 48px;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 20px;
		color: #fff;
	}

	h1 {
		margin: 0;
		font-size: 1.8rem;
	}

	.status {
		color: #aaa;
		margin: 0;
	}

	.spinner {
		width: 40px;
		height: 40px;
		border: 4px solid #333;
		border-top-color: #538d4e;
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}

	@keyframes spin {
		to { transform: rotate(360deg); }
	}

	.btn-cancel {
		padding: 10px 28px;
		background: transparent;
		color: #ccc;
		border: 1px solid #555;
		border-radius: 4px;
		cursor: pointer;
		font-size: 0.95rem;
		transition: border-color 0.2s;
	}

	.btn-cancel:hover {
		border-color: #fff;
		color: #fff;
	}
</style>
