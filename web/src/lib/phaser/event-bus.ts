// Typed pub/sub bus for Phaser ↔ Svelte communication.
// Intentionally tiny — no mitt dependency. Event names are string literals;
// consumers should define a const for each name to avoid typos.
// Convention: phaser→svelte events use kebab-case namespaced by scene,
// e.g. 'title:start', 'game:move', 'game:over'.

type Handler = (...args: unknown[]) => void;

class EventBus {
	private readonly listeners = new Map<string, Set<Handler>>();

	on(event: string, handler: Handler): void {
		let set = this.listeners.get(event);
		if (!set) {
			set = new Set();
			this.listeners.set(event, set);
		}
		set.add(handler);
	}

	off(event: string, handler: Handler): void {
		this.listeners.get(event)?.delete(handler);
	}

	emit(event: string, ...args: unknown[]): void {
		this.listeners.get(event)?.forEach((h) => h(...args));
	}
}

// Single shared instance used by all scenes and Svelte components.
export const eventBus = new EventBus();
