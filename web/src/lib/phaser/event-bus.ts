// Typed pub/sub bus for Phaser ↔ Svelte communication.
// Intentionally tiny — no mitt dependency. Event names are string literals;
// consumers should define a const for each name to avoid typos.
// Convention: phaser→svelte events use kebab-case namespaced by scene,
// e.g. 'title:start', 'game:move', 'game:over'.

import type { Color } from '$lib/game/wordle/colors';

/** Canonical map of every event name → its argument tuple. */
type Events = {
	/** Emitted by TitleScene when the user taps "Play". No arguments. */
	'title:start': [];
	/** Emitted by the play route after each server GAME_STATE reply. */
	'wordle:flip-row': [{ row: number; colors: Color[] }];
};

type Handler<K extends keyof Events> = (...args: Events[K]) => void;

class EventBus {
	// Map values are typed as Function[] because the per-key generic cannot be
	// captured in a single heterogeneous Map value type without a cast.
	// The public API remains fully typed; the cast is confined here.
	private readonly listeners = new Map<keyof Events, Function[]>(); // eslint-disable-line @typescript-eslint/no-unsafe-function-type

	on<K extends keyof Events>(event: K, handler: Handler<K>): void {
		const list = this.listeners.get(event) ?? [];
		list.push(handler);
		this.listeners.set(event, list);
	}

	off<K extends keyof Events>(event: K, handler: Handler<K>): void {
		const list = this.listeners.get(event);
		if (!list) return;
		const idx = list.indexOf(handler);
		if (idx !== -1) list.splice(idx, 1);
	}

	emit<K extends keyof Events>(event: K, ...args: Events[K]): void {
		const list = this.listeners.get(event);
		if (!list) return;
		for (const h of list) (h as (...a: unknown[]) => void)(...(args as unknown[]));
	}
}

// Single shared instance used by all scenes and Svelte components.
export const eventBus = new EventBus();
