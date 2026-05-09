<script lang="ts">
	// On-screen QWERTY keyboard. Tracks each letter's best colour state.
	// Emits a 'key' custom event with the letter or 'Enter'/'Backspace'.
	import type { Color } from '$lib/game/wordle/colors';

	interface Props {
		hints: Color[][];
		guesses: string[];
		onkey: (key: string) => void;
	}

	const { hints, guesses, onkey }: Props = $props();

	const ROWS = [
		['Q','W','E','R','T','Y','U','I','O','P'],
		['A','S','D','F','G','H','J','K','L'],
		['Enter','Z','X','C','V','B','N','M','Backspace']
	];

	// Color priority: green > yellow > gray > undefined.
	const colorPriority: Record<string, number> = { green: 3, yellow: 2, gray: 1 };

	/** Compute best-color-so-far for a single letter across all submitted hints. */
	function letterColor(letter: string): Color | null {
		let best: Color | null = null;
		let bestPriority = 0;
		for (let r = 0; r < guesses.length; r++) {
			for (let c = 0; c < guesses[r].length; c++) {
				if (guesses[r][c] === letter) {
					const color = hints[r]?.[c] ?? null;
					if (color) {
						const p = colorPriority[color] ?? 0;
						if (p > bestPriority) {
							bestPriority = p;
							best = color;
						}
					}
				}
			}
		}
		return best;
	}

	function keyClass(key: string): string {
		if (key === 'Enter' || key === 'Backspace') return 'key key--wide';
		const color = letterColor(key);
		switch (color) {
			case 'green':  return 'key key--green';
			case 'yellow': return 'key key--yellow';
			case 'gray':   return 'key key--gray';
			default:       return 'key';
		}
	}

	function keyLabel(key: string): string {
		if (key === 'Backspace') return '⌫';
		return key;
	}
</script>

<div class="keyboard" aria-label="On-screen keyboard">
	{#each ROWS as row}
		<div class="keyboard-row">
			{#each row as key}
				<button
					class={keyClass(key)}
					aria-label={key}
					onclick={() => onkey(key)}
				>
					{keyLabel(key)}
				</button>
			{/each}
		</div>
	{/each}
</div>

<style>
	.keyboard {
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding: 8px 0;
		user-select: none;
	}

	.keyboard-row {
		display: flex;
		justify-content: center;
		gap: 6px;
	}

	.key {
		height: 58px;
		min-width: 43px;
		padding: 0 6px;
		border: none;
		border-radius: 4px;
		background: #818384;
		color: #ffffff;
		font-family: monospace;
		font-size: 0.85rem;
		font-weight: bold;
		cursor: pointer;
		display: flex;
		align-items: center;
		justify-content: center;
	}

	.key--wide {
		min-width: 65px;
		font-size: 0.75rem;
	}

	.key--green  { background: #538d4e; }
	.key--yellow { background: #b59f3b; }
	.key--gray   { background: #3a3a3c; }

	.key:hover {
		filter: brightness(1.15);
	}

	.key:active {
		filter: brightness(0.9);
	}
</style>
