<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import SyncGameScene from '$lib/components/sync-game-scene.svelte';

	// Read match params from query string (set by /quick-match on QUEUE_MATCHED).
	const matchId = $derived($page.url.searchParams.get('matchId') ?? '');
	const seed = $derived(BigInt($page.url.searchParams.get('seed') ?? '0'));
	const opponentName = $derived($page.url.searchParams.get('opponent') ?? 'Opponent');

	// Guard: redirect home if no matchId.
	$effect(() => {
		if (!matchId) {
			goto('/');
		}
	});
</script>

{#if matchId}
	<SyncGameScene {matchId} {seed} {opponentName} />
{/if}
