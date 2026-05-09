<script lang="ts">
	// ResultsScreen is shown after a game reaches a terminal state.
	// It shows the outcome and CTA buttons depending on match context.
	// - solo daily (no matchId): shows "Challenge a friend" (share button)
	// - challenge match: shows pending/winner status

	import { goto } from '$app/navigation';
	import ShareButton from './share-button.svelte';

	type Props = {
		won: boolean;
		solution: string;
		/** Present when playing a challenge match. */
		matchId?: string;
		/** Present after AttemptSubmitAck with status "completed". */
		winnerUid?: string;
		/** Current user's Firebase UID — to detect if they won. */
		currentUid?: string;
		/** Share token returned by ChallengeCreateAck (solo daily only). */
		shareToken?: string;
		/** Emitted when user requests a challenge to be created. */
		onCreateChallenge?: () => void;
		/** True while waiting for AttemptSubmitAck. */
		submitting?: boolean;
	};

	let {
		won,
		solution,
		matchId,
		winnerUid,
		currentUid,
		shareToken,
		onCreateChallenge,
		submitting = false
	}: Props = $props();

	const isChallenge = $derived(!!matchId);
	const isPending = $derived(isChallenge && !winnerUid);
	const iWon = $derived(!!winnerUid && winnerUid === currentUid);
</script>

<div class="results-root" role="region" aria-label="Game result">
	<!-- Outcome headline -->
	{#if won}
		<p class="headline win">You solved it!</p>
	{:else}
		<p class="headline loss">Better luck next time</p>
	{/if}

	<p class="solution">Answer: <strong>{solution}</strong></p>

	<!-- Challenge match status -->
	{#if isChallenge}
		{#if submitting}
			<p class="match-status pending">Submitting your result…</p>
		{:else if isPending}
			<p class="match-status pending">Waiting for your opponent…</p>
		{:else if iWon}
			<p class="match-status winner">You won the challenge!</p>
		{:else}
			<p class="match-status loser">Your opponent won this one.</p>
		{/if}
	{/if}

	<!-- CTAs -->
	<div class="cta-row">
		{#if !isChallenge}
			<!-- Solo daily: offer to create challenge or show existing share token -->
			{#if shareToken}
				<ShareButton {shareToken} />
			{:else if onCreateChallenge}
				<button class="btn btn--primary" onclick={onCreateChallenge} disabled={submitting}>
					Challenge a friend
				</button>
			{/if}
		{/if}

		<button class="btn btn--secondary" onclick={() => goto('/leaderboard')}>
			View leaderboard
		</button>

		<button class="btn btn--ghost" onclick={() => goto('/')}>
			Home
		</button>
	</div>
</div>

<style>
	.results-root {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 12px;
		padding: 24px 20px;
		border-radius: 8px;
		background: #1c1c1e;
		border: 1px solid #3a3a3c;
		min-width: 280px;
	}

	.headline {
		font-size: 1.4rem;
		font-weight: bold;
		margin: 0;
		letter-spacing: 0.05em;
	}
	.headline.win  { color: #538d4e; }
	.headline.loss { color: #818384; }

	.solution {
		margin: 0;
		font-size: 1rem;
		color: #c8c8c8;
	}

	.match-status {
		font-size: 0.9rem;
		margin: 0;
		padding: 6px 14px;
		border-radius: 4px;
	}
	.match-status.pending { background: #2a2a2c; color: #818384; }
	.match-status.winner  { background: #1a3a1a; color: #538d4e; }
	.match-status.loser   { background: #2a1a1a; color: #ff6b6b; }

	.cta-row {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 8px;
		margin-top: 4px;
		width: 100%;
	}

	.btn {
		padding: 9px 20px;
		border: none;
		border-radius: 4px;
		font-family: monospace;
		font-size: 0.9rem;
		cursor: pointer;
		width: 100%;
		max-width: 240px;
		transition: opacity 0.15s;
	}
	.btn:disabled { opacity: 0.5; cursor: default; }
	.btn:not(:disabled):hover { opacity: 0.85; }

	.btn--primary   { background: #538d4e; color: #fff; }
	.btn--secondary { background: #3a3a3c; color: #fff; }
	.btn--ghost     { background: transparent; color: #818384; border: 1px solid #3a3a3c; }
</style>
