<script lang="ts">
	import { authError } from '$lib/auth-error-store';
	import { authUser, idToken } from '$lib/auth-store';
	import { connect } from '$lib/ws';
	import { get } from 'svelte/store';

	async function retry() {
		authError.set(null);
		const u = get(authUser);
		if (!u) return;
		try {
			connect(await idToken(true));
		} catch {
			authError.set({
				kind: 'connection_failed',
				message: 'Connection failed; try again'
			});
		}
	}
</script>

{#if $authError}
	<div class="auth-toast" role="alert">
		<span class="msg">{$authError.message}</span>
		<button onclick={retry}>Retry</button>
	</div>
{/if}

<style>
	.auth-toast {
		position: fixed;
		bottom: 1rem;
		right: 1rem;
		display: flex;
		align-items: center;
		gap: 0.75rem;
		padding: 0.5rem 1rem;
		background: #2e0f0f;
		color: #dd4444;
		border: 1px solid #dd4444;
		border-radius: 6px;
		font-family: monospace;
		font-size: 0.85rem;
		z-index: 2000;
	}

	button {
		background: #dd4444;
		color: #fff;
		border: none;
		border-radius: 4px;
		padding: 0.2rem 0.6rem;
		font-family: monospace;
		font-size: 0.8rem;
		cursor: pointer;
	}

	button:hover {
		background: #ff6666;
	}
</style>
