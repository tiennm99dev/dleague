<script lang="ts">
	// ShareButton copies the challenge link to the clipboard.
	// shareToken: the UUID v4 returned by ChallengeCreateAck.

	type Props = { shareToken: string };
	let { shareToken }: Props = $props();

	let copied = $state(false);
	let copyError = $state('');

	async function copyLink(): Promise<void> {
		const url = `${window.location.origin}/m/${shareToken}`;
		try {
			await navigator.clipboard.writeText(url);
			copied = true;
			copyError = '';
			setTimeout(() => (copied = false), 2000);
		} catch {
			copyError = 'Copy failed — try manually: ' + url;
		}
	}
</script>

<div class="share-wrap">
	<button class="share-btn" onclick={copyLink} disabled={copied}>
		{copied ? '✓ Copied!' : 'Challenge a friend'}
	</button>
	{#if copyError}
		<p class="copy-error">{copyError}</p>
	{/if}
</div>

<style>
	.share-wrap {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 6px;
	}

	.share-btn {
		padding: 10px 24px;
		background: #538d4e;
		border: none;
		border-radius: 4px;
		color: #ffffff;
		font-family: monospace;
		font-size: 0.95rem;
		cursor: pointer;
		transition: opacity 0.15s;
	}

	.share-btn:disabled {
		opacity: 0.7;
		cursor: default;
	}

	.share-btn:not(:disabled):hover {
		opacity: 0.9;
	}

	.copy-error {
		font-size: 0.8rem;
		color: #ff6b6b;
		margin: 0;
		word-break: break-all;
	}
</style>
