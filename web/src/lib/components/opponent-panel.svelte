<script lang="ts">
	import type { Color } from '$lib/pb/dleague/v1/wordle_pb';

	// Each entry is the color array for one opponent attempt (colors only, no letters).
	type ColorRow = Color[];

	interface Props {
		rows: ColorRow[];
		maxAttempts?: number;
		wordLength?: number;
		opponentName?: string;
	}

	let {
		rows = [],
		maxAttempts = 6,
		wordLength = 5,
		opponentName = 'Opponent'
	}: Props = $props();

	// Color enum values: GRAY=1, YELLOW=2, GREEN=3 (0=unspecified/empty).
	function colorClass(c: Color): string {
		switch (c) {
			case 3:
				return 'tile-green';
			case 2:
				return 'tile-yellow';
			case 1:
				return 'tile-gray';
			default:
				return 'tile-empty';
		}
	}

	// Build a grid of maxAttempts rows × wordLength cols.
	// Filled rows show colors; empty rows show placeholders.
	function buildGrid(): Color[][] {
		const grid: Color[][] = [];
		for (let r = 0; r < maxAttempts; r++) {
			if (r < rows.length) {
				grid.push(rows[r]);
			} else {
				grid.push(new Array<Color>(wordLength).fill(0 as Color));
			}
		}
		return grid;
	}
</script>

<div class="opponent-panel">
	<h3 class="opponent-name">{opponentName}</h3>
	<div class="grid">
		{#each buildGrid() as row, rowIdx}
			<div class="row">
				{#each row as color, colIdx}
					<div
						class="tile {colorClass(color)}"
						aria-label={`row ${rowIdx + 1} tile ${colIdx + 1}`}
					></div>
				{/each}
			</div>
		{/each}
	</div>
</div>

<style>
	.opponent-panel {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 8px;
	}

	.opponent-name {
		color: #ccc;
		font-size: 0.9rem;
		margin: 0;
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.grid {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.row {
		display: flex;
		gap: 4px;
	}

	.tile {
		width: 28px;
		height: 28px;
		border-radius: 3px;
	}

	.tile-empty {
		background: #3a3a3c;
		border: 1px solid #555;
	}

	.tile-gray {
		background: #3a3a3c;
	}

	.tile-yellow {
		background: #b59f3b;
	}

	.tile-green {
		background: #538d4e;
	}
</style>
