<script lang="ts">
	// Top-right badge showing WebSocket connection state.
	// When disconnected, shows a Reconnect button that re-opens the socket.
	import { connectionState, connect } from '$lib/ws';
	import { idToken } from '$lib/auth-store';

	const labels: Record<string, string> = {
		connected: 'Connected',
		connecting: 'Connecting…',
		disconnected: 'Disconnected'
	};

	let reconnecting = $state(false);

	async function handleReconnect(): Promise<void> {
		if (reconnecting) return;
		reconnecting = true;
		try {
			await connect(await idToken());
		} catch {
			// AuthErrorToast surfaces auth failures; nothing to do here.
		} finally {
			reconnecting = false;
		}
	}
</script>

<div
	class="status-badge {$connectionState}"
	aria-label="WebSocket status: {labels[$connectionState]}"
>
	<span class="dot"></span>
	{labels[$connectionState]}
	{#if $connectionState === 'disconnected'}
		<button
			class="reconnect-btn"
			onclick={handleReconnect}
			disabled={reconnecting}
		>Reconnect</button>
	{/if}
</div>

<style>
	.status-badge {
		position: fixed;
		top: 0.75rem;
		right: 0.75rem;
		display: flex;
		align-items: center;
		gap: 0.4rem;
		padding: 0.3rem 0.7rem;
		border-radius: 999px;
		font-family: monospace;
		font-size: 0.8rem;
		z-index: 1000;
	}

	.dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
	}

	/* Color variants by state class */
	.connected {
		background: #0f2e1a;
		color: #44dd88;
		border: 1px solid #44dd88;
	}
	.connected .dot {
		background: #44dd88;
	}

	.connecting {
		background: #2e2a0f;
		color: #ddcc44;
		border: 1px solid #ddcc44;
	}
	.connecting .dot {
		background: #ddcc44;
	}

	.disconnected {
		background: #2e0f0f;
		color: #dd4444;
		border: 1px solid #dd4444;
	}
	.disconnected .dot {
		background: #dd4444;
	}

	.reconnect-btn {
		background: #dd4444;
		color: #fff;
		border: none;
		border-radius: 4px;
		padding: 0.15rem 0.5rem;
		font-family: monospace;
		font-size: 0.75rem;
		cursor: pointer;
		margin-left: 0.25rem;
	}

	.reconnect-btn:hover {
		filter: brightness(1.15);
	}
</style>
