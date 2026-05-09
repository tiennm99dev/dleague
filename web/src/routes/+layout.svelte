<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { authUser, idToken } from '$lib/auth-store';
	import { initFirebase } from '$lib/firebase';
	import SignIn from '$lib/components/sign-in.svelte';
	import ConnectionStatus from '$lib/components/connection-status.svelte';
	import {
		connectionState,
		connect,
		disconnect,
		sendMatchRejoin
	} from '$lib/ws';
	import { matchRejoinStore } from '$lib/match-rejoin-store';

	let { children } = $props();

	// Track whether Firebase has emitted its first auth-state event.
	let authResolved = $state(false);

	onMount(() => {
		const unsubscribe = initFirebase();

		// Hoist WS lifecycle: connect on sign-in, disconnect on sign-out.
		let connected = false;
		const unsub = authUser.subscribe(async (u) => {
			authResolved = true;
			if (u && !connected) {
				try {
					connect(await idToken());
					connected = true;
				} catch (e) {
					console.warn('ws: connect failed', e);
				}
			} else if (!u && connected) {
				disconnect();
				connected = false;
			}
		});

		// Reconnect logic: when WS transitions to 'connected', check if there
		// is an active match in sessionStorage and attempt to rejoin.
		const unsubConn = connectionState.subscribe((state) => {
			if (state !== 'connected') return;
			const activeMatchID = sessionStorage.getItem('activeMatchID');
			if (!activeMatchID) return;

			sendMatchRejoin(activeMatchID)
				.then((ack) => {
					// Set rejoin store BEFORE navigating so sync route can read it.
					matchRejoinStore.set({
						ownState: ack.ownState ?? undefined,
						opponentHints: ack.opponentHints
					});

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
			if (connected) disconnect();
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
