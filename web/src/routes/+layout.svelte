<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { authUser } from '$lib/auth-store';
	import { initFirebase } from '$lib/firebase';
	import SignIn from '$lib/components/sign-in.svelte';
	import ConnectionStatus from '$lib/components/connection-status.svelte';
	import {
		connectionState,
		sendMatchRejoin,
		onMatchRejoinAck,
		removeHandler
	} from '$lib/ws';
	import { MessageType } from '$lib/pb/dleague/v1/envelope_pb';

	let { children } = $props();

	// Track whether Firebase has emitted its first auth-state event.
	let authResolved = $state(false);

	onMount(() => {
		const unsubscribe = initFirebase();
		const unsub = authUser.subscribe(() => {
			authResolved = true;
		});

		// Reconnect logic: when WS transitions to 'connected', check if there
		// is an active match in sessionStorage and attempt to rejoin.
		const unsubConn = connectionState.subscribe((state) => {
			if (state !== 'connected') return;
			const activeMatchID = sessionStorage.getItem('activeMatchID');
			if (!activeMatchID) return;

			sendMatchRejoin(activeMatchID)
				.then((ack) => {
					// Successfully rejoined: stay on /sync page (or navigate if needed).
					const currentPath = window.location.pathname;
					if (!currentPath.startsWith('/sync')) {
						const seed = sessionStorage.getItem('activeSeed') ?? '0';
						const opponent = sessionStorage.getItem('activeOpponent') ?? 'Opponent';
						goto(
							`/sync?matchId=${encodeURIComponent(ack.matchId)}&seed=${seed}&opponent=${encodeURIComponent(opponent)}`
						);
					}
				})
				.catch(() => {
					// Rejoin failed (match resolved, stale ID, etc.) — clear and go home.
					sessionStorage.removeItem('activeMatchID');
					sessionStorage.removeItem('activeSeed');
					sessionStorage.removeItem('activeOpponent');
					const currentPath = window.location.pathname;
					if (currentPath.startsWith('/sync') || currentPath.startsWith('/quick-match')) {
						goto('/');
					}
				});
		});

		return () => {
			unsubscribe();
			unsub();
			unsubConn();
			removeHandler(MessageType.MATCH_REJOIN_ACK);
		};
	});
</script>

{#if authResolved}
	{#if $authUser}
		<ConnectionStatus />
		{@render children()}
	{:else}
		<SignIn />
	{/if}
{/if}
