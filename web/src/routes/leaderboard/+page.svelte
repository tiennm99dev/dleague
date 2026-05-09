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
	import { idToken } from '$lib/auth-store';
	import { connect, disconnect, sendRequest, connectionState } from '$lib/ws';

	// ── State ─────────────────────────────────────────────────────────────────

	let rankings: LeaderboardEntry[] = $state([]);
	let loading = $state(true);
	let errorMsg = $state('');

	// ── Query ─────────────────────────────────────────────────────────────────

	async function fetchLeaderboard(): Promise<void> {
		loading = true;
		errorMsg = '';
		try {
			const query = create(LeaderboardQuerySchema, { gameId: 'wordle', period: 'daily' });
			const payload = toBinary(LeaderboardQuerySchema, query);
			const respBytes = await sendRequest(MessageType.LEADERBOARD_QUERY, payload, 8_000);
			const snapshot = fromBinary(LeaderboardSnapshotSchema, respBytes);
			rankings = snapshot.rankings;
		} catch (err) {
			errorMsg = err instanceof Error ? err.message : 'Failed to load leaderboard';
		} finally {
			loading = false;
		}
	}

	/** Formats milliseconds as m:ss. */
	function formatTime(ms: number): string {
		const totalSec = Math.floor(ms / 1000);
		const min = Math.floor(totalSec / 60);
		const sec = totalSec % 60;
		return `${min}:${sec.toString().padStart(2, '0')}`;
	}

	// ── Lifecycle ─────────────────────────────────────────────────────────────

	function onFocus(): void {
		void fetchLeaderboard();
	}

	onMount(async () => {
		try {
			const token = await idToken();
			connect(token);
		} catch {
			connect('');
		}

		// Wait for connection then fetch.
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
		disconnect();
	});
</script>

<svelte:head>
	<title>Dleague — Leaderboard</title>
</svelte:head>

<main class="lb-root">
	<header class="lb-header">
		<button class="back-btn" onclick={() => goto('/')}>← Back</button>
		<h1>Daily Leaderboard</h1>
		<button class="refresh-btn" onclick={() => void fetchLeaderboard()} disabled={loading}>
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
	{:else if rankings.length === 0}
		<p class="empty">No entries yet for today. Play the daily puzzle!</p>
	{:else}
		<div class="table-wrap">
			<table class="lb-table" aria-label="Daily leaderboard">
				<thead>
					<tr>
						<th scope="col">#</th>
						<th scope="col">Player</th>
						<th scope="col">Guesses</th>
						<th scope="col">Time</th>
					</tr>
				</thead>
				<tbody>
					{#each rankings as entry (entry.uid)}
						<tr class="row" class:top3={entry.rank <= 3}>
							<td class="rank">{entry.rank}</td>
							<td class="name">{entry.displayName || entry.uid.slice(0, 8)}</td>
							<td class="guesses">{entry.attempts}</td>
							<td class="time">{formatTime(entry.timeMs)}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
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

	.refresh-btn:disabled { opacity: 0.4; cursor: default; }

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
		to { transform: rotate(360deg); }
	}

	.empty {
		color: #818384;
		font-size: 0.9rem;
		margin-top: 40px;
		text-align: center;
		padding: 0 16px;
	}

	.table-wrap {
		width: 100%;
		max-width: 500px;
		padding: 0 8px;
		overflow-x: auto;
	}

	.lb-table {
		width: 100%;
		border-collapse: collapse;
		font-size: 0.9rem;
	}

	.lb-table th {
		text-align: left;
		padding: 6px 10px;
		color: #818384;
		font-weight: normal;
		border-bottom: 1px solid #3a3a3c;
	}

	.lb-table td {
		padding: 9px 10px;
		border-bottom: 1px solid #2a2a2c;
	}

	.row.top3 .name {
		color: #538d4e;
		font-weight: bold;
	}

	.rank   { width: 32px; color: #818384; }
	.guesses,
	.time   { text-align: right; color: #c8c8c8; }
</style>
