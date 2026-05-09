// Title scene — replicates the layout from the deleted client/internal/scene/title.go.
// Shows the "DLEAGUE" title and a "Start" button. On click, emits 'title:start'
// on the shared EventBus so the Svelte +page.svelte can navigate to /play.
import Phaser from 'phaser';
import { eventBus } from '../event-bus';

const TITLE_TEXT = 'DLEAGUE';
const BUTTON_TEXT = 'Start';
const CANVAS_CENTER_X = 400;

export class TitleScene extends Phaser.Scene {
	constructor() {
		super({ key: 'TitleScene' });
	}

	create(): void {
		const { height } = this.scale;

		// ── Title ──────────────────────────────────────────────────────────────
		this.add
			.text(CANVAS_CENTER_X, height * 0.35, TITLE_TEXT, {
				fontFamily: 'monospace',
				fontSize: '72px',
				color: '#e0e0ff',
				stroke: '#6060cc',
				strokeThickness: 4
			})
			.setOrigin(0.5);

		// ── Start button ──────────────────────────────────────────────────────
		const btnBg = this.add
			.rectangle(CANVAS_CENTER_X, height * 0.6, 200, 60, 0x4444cc)
			.setInteractive({ useHandCursor: true });

		const btnLabel = this.add
			.text(CANVAS_CENTER_X, height * 0.6, BUTTON_TEXT, {
				fontFamily: 'monospace',
				fontSize: '28px',
				color: '#ffffff'
			})
			.setOrigin(0.5);

		// Hover feedback
		btnBg.on(Phaser.Input.Events.POINTER_OVER, () => {
			btnBg.setFillStyle(0x6666ee);
			btnLabel.setColor('#ffff88');
		});
		btnBg.on(Phaser.Input.Events.POINTER_OUT, () => {
			btnBg.setFillStyle(0x4444cc);
			btnLabel.setColor('#ffffff');
		});

		// Click → signal Svelte to navigate
		btnBg.on(Phaser.Input.Events.POINTER_DOWN, () => {
			eventBus.emit('title:start');
		});

		// ── Subtitle ─────────────────────────────────────────────────────────
		this.add
			.text(CANVAS_CENTER_X, height * 0.8, 'daily word-game league', {
				fontFamily: 'monospace',
				fontSize: '18px',
				color: '#8888cc'
			})
			.setOrigin(0.5);
	}
}
