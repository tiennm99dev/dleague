<script lang="ts">
	// Top-right badge showing WebSocket connection state.
	// Subscribes to the connectionState writable from ws.ts.
	import { connectionState } from '$lib/ws';

	const labels: Record<string, string> = {
		connected: 'Connected',
		connecting: 'Connecting…',
		disconnected: 'Disconnected'
	};
</script>

<div class="status-badge {$connectionState}" aria-label="WebSocket status: {labels[$connectionState]}">
	<span class="dot"></span>
	{labels[$connectionState]}
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
		pointer-events: none;
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
</style>
