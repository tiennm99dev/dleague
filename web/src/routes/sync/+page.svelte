<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { get } from 'svelte/store';
	import SyncGameScene from '$lib/components/sync-game-scene.svelte';
	import { matchRejoinStore } from '$lib/match-rejoin-store';
	import type { WordleState, WordleHint } from '$lib/pb/dleague/v1/wordle_pb';

	// Read match params from query string (set by /quick-match on QUEUE_MATCHED).
	const matchId = $derived($page.url.searchParams.get('matchId') ?? '');
	const seed = $derived(BigInt($page.url.searchParams.get('seed') ?? '0'));
	const opponentName = $derived(
		$page.url.searchParams.get('opponent') ?? 'Opponent'
	);

	// Rejoin state (set by layout before navigation on MATCH_REJOIN_ACK).
	let initialState = $state<WordleState | undefined>(undefined);
	let initialOpponentHints = $state<WordleHint[]>([]);

	onMount(() => {
		const rejoin = get(matchRejoinStore);
		if (rejoin) {
			initialState = rejoin.ownState;
			initialOpponentHints = rejoin.opponentHints ?? [];
			matchRejoinStore.set(null);
		}
	});

	// Guard: redirect home if no matchId.
	$effect(() => {
		if (!matchId) {
			goto('/');
		}
	});
</script>

{#if matchId}
	<SyncGameScene
		{matchId}
		{seed}
		{opponentName}
		{initialState}
		{initialOpponentHints}
	/>
{/if}
