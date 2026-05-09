// Phaser scene that overlays the Svelte board and runs tile-flip animations.
// It does not own game state — it only animates in response to EventBus events.
// The Svelte board renders DOM tiles; this scene draws matching Phaser rectangles
// on top, flips them over ~600ms, then hides itself so the DOM tile shows through.
import Phaser from 'phaser';
import { eventBus } from '../event-bus';
import type { Color } from '$lib/game/wordle/colors';

// EventBus payload emitted by the play route after a server GAME_STATE reply.
export interface FlipRowPayload {
	row: number;
	colors: Color[];  // length 5
}

const TILE_SIZE    = 56;
const TILE_GAP     = 6;
const BOARD_PAD    = 12;
const COLS         = 5;
const FLIP_HALF_MS = 150; // half-flip duration in ms (total flip = 4 × half)

const COLOR_MAP: Record<Color, number> = {
	green:  0x538d4e,
	yellow: 0xb59f3b,
	gray:   0x3a3a3c,
};

export class WordleScene extends Phaser.Scene {
	// Bound handler stored so we can call eventBus.off() in shutdown. Phase 07 M5 fix.
	private readonly flipRowHandler = (payload: unknown): void => {
		const { row, colors } = payload as FlipRowPayload;
		this.flipRow(row, colors);
	};

	constructor() {
		super({ key: 'WordleScene' });
	}

	create(): void {
		// Listen for flip-row events emitted after each server GAME_STATE reply.
		eventBus.on('wordle:flip-row', this.flipRowHandler);
	}

	/**
	 * shutdown is called by Phaser when the scene is stopped or destroyed.
	 * Remove the EventBus listener to prevent memory leaks and stale callbacks
	 * if the scene is restarted. Phase 07 M5 fix.
	 */
	shutdown(): void {
		eventBus.off('wordle:flip-row', this.flipRowHandler);
	}

	/**
	 * flipRow animates a Y-axis scale collapse → recolor → expand for each tile
	 * in the given row, staggered left-to-right for a wave effect.
	 */
	private flipRow(row: number, colors: Color[]): void {
		for (let col = 0; col < COLS; col++) {
			const x = BOARD_PAD + col * (TILE_SIZE + TILE_GAP) + TILE_SIZE / 2;
			const y = BOARD_PAD + row * (TILE_SIZE + TILE_GAP) + TILE_SIZE / 2;

			const rect = this.add.rectangle(x, y, TILE_SIZE, TILE_SIZE, 0x121213);
			rect.setDepth(10);

			const delay = col * 100; // stagger per column

			// Phase 1: scale Y 1 → 0 (fold down)
			this.tweens.add({
				targets: rect,
				scaleY: 0,
				duration: FLIP_HALF_MS,
				delay,
				ease: 'Linear',
				onComplete: () => {
					// Recolor at midpoint (tile is invisible at scaleY=0)
					rect.setFillStyle(COLOR_MAP[colors[col]] ?? 0x3a3a3c);

					// Phase 2: scale Y 0 → 1 (unfold with new color)
					this.tweens.add({
						targets: rect,
						scaleY: 1,
						duration: FLIP_HALF_MS,
						ease: 'Linear',
						onComplete: () => {
							// Hold briefly then fade out so DOM tile shows through.
							this.tweens.add({
								targets: rect,
								alpha: 0,
								duration: 200,
								delay: 300,
								ease: 'Linear',
								onComplete: () => rect.destroy()
							});
						}
					});
				}
			});
		}
	}
}
