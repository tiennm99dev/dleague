<script lang="ts">
	// LeaderboardTable renders a ranked list of players.
	// rankings: array of LeaderboardEntry from the LeaderboardSnapshot proto.

	import type { LeaderboardEntry } from '$lib/pb/dleague/v1/match_pb';
	import { formatTime } from '$lib/format-time';

	type Props = { rankings: LeaderboardEntry[] };
	let { rankings }: Props = $props();
</script>

<div class="table-wrap">
	{#if rankings.length === 0}
		<p class="empty">No entries yet. Be the first!</p>
	{:else}
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
						<td class="attempts">{entry.attempts}</td>
						<td class="time">{formatTime(entry.timeMs)}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}
</div>

<style>
	.table-wrap {
		width: 100%;
		max-width: 480px;
		overflow-x: auto;
	}

	.empty {
		text-align: center;
		color: #818384;
		font-size: 0.9rem;
	}

	.lb-table {
		width: 100%;
		border-collapse: collapse;
		font-family: monospace;
		font-size: 0.9rem;
		color: #ffffff;
	}

	.lb-table th {
		text-align: left;
		padding: 6px 10px;
		color: #818384;
		font-weight: normal;
		border-bottom: 1px solid #3a3a3c;
	}

	.lb-table td {
		padding: 8px 10px;
		border-bottom: 1px solid #3a3a3c;
	}

	.row.top3 .name {
		color: #538d4e;
		font-weight: bold;
	}

	.rank {
		width: 32px;
		color: #818384;
	}

	.attempts,
	.time {
		text-align: right;
	}
</style>
