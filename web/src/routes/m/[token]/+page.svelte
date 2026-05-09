<script lang="ts">
	// Challenge accept page: /m/<share_token>
	// On mount, sends CHALLENGE_JOIN over WS. On success, navigates to /play
	// with the match ID and seed as query params.
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { create, toBinary, fromBinary } from '@bufbuild/protobuf';
	import { MessageType } from '$lib/pb/dleague/v1/envelope_pb';
	import {
		ChallengeJoinSchema,
		ChallengeJoinAckSchema
	} from '$lib/pb/dleague/v1/match_pb';
	import { sendRequest, connectionState } from '$lib/ws';

	// ── State ────────────────────────────────────────────────────────────────

	let status: 'connecting' | 'joining' | 'error' | 'redirecting' =
		$state('connecting');
	let errorMsg = $state('');

	// ── Join logic ────────────────────────────────────────────────────────────

	async function joinChallenge(token: string): Promise<void> {
		// Wait for WS to be connected (it connects in onMount).
		await waitForConnection();

		status = 'joining';
		const joinMsg = create(ChallengeJoinSchema, { shareToken: token });
		const payload = toBinary(ChallengeJoinSchema, joinMsg);

		try {
			const respBytes = await sendRequest(
				MessageType.CHALLENGE_JOIN,
				payload,
				10_000
			);
			const ack = fromBinary(ChallengeJoinAckSchema, respBytes);
			status = 'redirecting';
			await goto(`/play?match=${ack.matchId}&seed=${ack.seed}`);
		} catch (err) {
			status = 'error';
			const msg = err instanceof Error ? err.message : String(err);
			if (msg.includes('409') || msg.toLowerCase().includes('taken')) {
				errorMsg = 'This challenge link has already been used.';
			} else if (
				msg.includes('404') ||
				msg.toLowerCase().includes('not found')
			) {
				errorMsg = 'Challenge not found or expired.';
			} else if (msg.toLowerCase().includes('own challenge')) {
				errorMsg = "You can't join your own challenge.";
			} else {
				errorMsg = 'Failed to join challenge. Please try again.';
			}
		}
	}

	/** Resolves when WS reaches 'connected'; rejects after 10 s. */
	function waitForConnection(): Promise<void> {
		return new Promise((resolve, reject) => {
			const timer = setTimeout(() => {
				unsub();
				reject(new Error('WebSocket connection timeout'));
			}, 10_000);

			const unsub = connectionState.subscribe((s) => {
				if (s === 'connected') {
					clearTimeout(timer);
					unsub();
					resolve();
				}
			});
		});
	}

	// ── Lifecycle ─────────────────────────────────────────────────────────────

	onMount(async () => {
		const token = $page.params.token;
		if (!token) {
			status = 'error';
			errorMsg = 'Invalid challenge link.';
			return;
		}

		await joinChallenge(token);
	});
</script>

<svelte:head>
	<title>Dleague — Join Challenge</title>
</svelte:head>

<main class="accept-root">
	{#if status === 'connecting' || status === 'joining'}
		<div class="spinner" aria-label="Joining challenge…"></div>
		<p class="status-text">
			{status === 'connecting' ? 'Connecting…' : 'Joining challenge…'}
		</p>
	{:else if status === 'redirecting'}
		<p class="status-text">Starting game…</p>
	{:else if status === 'error'}
		<div class="error-card">
			<p class="error-msg">{errorMsg}</p>
			<a href="/" class="home-link">← Back to home</a>
		</div>
	{/if}
</main>

<style>
	.accept-root {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		min-height: 100vh;
		background: #121213;
		color: #ffffff;
		font-family: monospace;
		gap: 16px;
	}

	.spinner {
		width: 40px;
		height: 40px;
		border: 4px solid #3a3a3c;
		border-top-color: #538d4e;
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	.status-text {
		color: #818384;
		font-size: 0.95rem;
	}

	.error-card {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 16px;
		padding: 32px;
		border: 1px solid #3a3a3c;
		border-radius: 8px;
		max-width: 360px;
		text-align: center;
	}

	.error-msg {
		color: #ff6b6b;
		font-size: 1rem;
		margin: 0;
	}

	.home-link {
		color: #538d4e;
		text-decoration: none;
		font-size: 0.9rem;
	}

	.home-link:hover {
		text-decoration: underline;
	}
</style>
