<script lang="ts">
	// 5×6 Wordle grid component.
	// Props are read-only data; no WS logic here — parent owns the connection.
	import type { Color } from '$lib/game/wordle/colors';

	interface Props {
		guesses: string[];
		hints: Color[][];
		currentInput: string;
	}

	const { guesses, hints, currentInput }: Props = $props();

	const ROWS = 6;
	const COLS = 5;

	/** Return the display letter for a given row/col position. */
	function getLetter(row: number, col: number): string {
		if (row < guesses.length) return guesses[row][col] ?? '';
		if (row === guesses.length) return currentInput[col] ?? '';
		return '';
	}

	/** Return the tile CSS class based on hint state. */
	function tileClass(row: number, col: number): string {
		if (row >= guesses.length) return 'tile tile--empty';
		const color: Color | undefined = hints[row]?.[col];
		switch (color) {
			case 'green':  return 'tile tile--green';
			case 'yellow': return 'tile tile--yellow';
			default:       return 'tile tile--gray';
		}
	}
</script>

<div class="board" role="region" aria-label="Wordle board" aria-live="polite">
	{#each { length: ROWS } as _, row}
		<div class="row">
			{#each { length: COLS } as _, col}
				{#if getLetter(row, col)}
					<div class={tileClass(row, col)} aria-label={getLetter(row, col)}>
						{getLetter(row, col)}
					</div>
				{:else}
					<div class={tileClass(row, col)} aria-hidden="true"></div>
				{/if}
			{/each}
		</div>
	{/each}
</div>

<style>
	.board {
		display: flex;
		flex-direction: column;
		gap: 6px;
		padding: 12px;
	}

	.row {
		display: flex;
		gap: 6px;
	}

	.tile {
		width: 56px;
		height: 56px;
		display: flex;
		align-items: center;
		justify-content: center;
		font-family: monospace;
		font-size: 2rem;
		font-weight: bold;
		text-transform: uppercase;
		border: 2px solid #565758;
		color: #ffffff;
		user-select: none;
	}

	.tile--empty {
		border-color: #565758;
		background: transparent;
		color: #ffffff;
	}

	.tile--gray {
		background: #3a3a3c;
		border-color: #3a3a3c;
	}

	.tile--yellow {
		background: #b59f3b;
		border-color: #b59f3b;
	}

	.tile--green {
		background: #538d4e;
		border-color: #538d4e;
	}
</style>
