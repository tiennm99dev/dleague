<script lang="ts">
	// Daily leaderboard page.
	// Sends LEADERBOARD_QUERY on mount and on window focus.
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { create, toBinary, fromBinary } from '@bufbuild/protobuf';
	import { MessageType } from '$lib/pb/dleague/v1/envelope_pb';
	import {
		LeaderboardQuerySchema,
		LeaderboardSnapshotSchema
	} from '$lib/pb/dleague/v1/match_pb';
	import type { LeaderboardEntry } from '$lib/pb/dleague/v1/match_pb';
	import { connectionState, sendRequest } from '$lib/ws';
	import LeaderboardTable from '$lib/components/leaderboard-table.svelte';

	// ── State ─────────────────────────────────────────────────────────────────

	let rankings: LeaderboardEntry[] = $state([]);
	let loading = $state(true);
	let errorMsg = $state('');

	// ── Query ─────────────────────────────────────────────────────────────────

	async function fetchLeaderboard(): Promise<void> {
		loading = true;
		errorMsg = '';
		try {
			const query = create(LeaderboardQuerySchema, {
				gameId: 'wordle',
				period: 'daily'
			});
			const payload = toBinary(LeaderboardQuerySchema, query);
			const respBytes = await sendRequest(
				MessageType.LEADERBOARD_QUERY,
				payload,
				8_000
			);
			const snapshot = fromBinary(LeaderboardSnapshotSchema, respBytes);
			rankings = snapshot.rankings;
		} catch (err) {
			errorMsg =
				err instanceof Error ? err.message : 'Failed to load leaderboard';
		} finally {
			loading = false;
		}
	}

	// ── Lifecycle ─────────────────────────────────────────────────────────────

	function onFocus(): void {
		void fetchLeaderboard();
	}

	onMount(() => {
		// Wait for connection (established by layout) then fetch.
		const unsub = connectionState.subscribe((s) => {
			if (s === 'connected') {
				unsub();
				void fetchLeaderboard();
			}
		});

		window.addEventListener('focus', onFocus);
	});

	onDestroy(() => {
		window.removeEventListener('focus', onFocus);
	});
</script>

<svelte:head>
	<title>Dleague — Leaderboard</title>
</svelte:head>

<main class="lb-root">
	<header class="lb-header">
		<button class="back-btn" onclick={() => goto('/')}>← Back</button>
		<h1>Daily Leaderboard</h1>
		<button
			class="refresh-btn"
			onclick={() => void fetchLeaderboard()}
			disabled={loading}
		>
			{loading ? '…' : '↻'}
		</button>
	</header>

	{#if errorMsg}
		<div class="toast-error" role="alert">{errorMsg}</div>
	{/if}

	{#if loading && rankings.length === 0}
		<div class="loading-row">
			<div class="spinner" aria-label="Loading…"></div>
		</div>
	{:else}
		<LeaderboardTable {rankings} />
	{/if}
</main>

<style>
	.lb-root {
		display: flex;
		flex-direction: column;
		align-items: center;
		min-height: 100vh;
		background: #121213;
		color: #ffffff;
		font-family: monospace;
		gap: 12px;
		padding-bottom: 32px;
	}

	.lb-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		width: 100%;
		max-width: 500px;
		padding: 12px 16px;
		border-bottom: 1px solid #3a3a3c;
	}

	.lb-header h1 {
		font-size: 1.3rem;
		letter-spacing: 0.08em;
		margin: 0;
	}

	.back-btn,
	.refresh-btn {
		background: none;
		border: none;
		color: #ffffff;
		cursor: pointer;
		font-family: monospace;
		font-size: 1rem;
		padding: 4px 8px;
	}

	.refresh-btn:disabled {
		opacity: 0.4;
		cursor: default;
	}

	.toast-error {
		background: #b00020;
		color: #fff;
		padding: 8px 16px;
		border-radius: 4px;
		font-size: 0.85rem;
	}

	.loading-row {
		margin-top: 40px;
	}

	.spinner {
		width: 36px;
		height: 36px;
		border: 4px solid #3a3a3c;
		border-top-color: #538d4e;
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
</style>
