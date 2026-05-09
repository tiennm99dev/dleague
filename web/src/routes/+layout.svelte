<script lang="ts">
	import { onMount } from 'svelte';
	import { authUser } from '$lib/auth-store';
	import { initFirebase } from '$lib/firebase';
	import SignIn from '$lib/components/sign-in.svelte';
	import ConnectionStatus from '$lib/components/connection-status.svelte';

	let { children } = $props();

	// Track whether Firebase has emitted its first auth-state event.
	// Before the first event, show nothing to avoid flash of sign-in UI.
	let authResolved = $state(false);

	onMount(() => {
		// initFirebase is idempotent; safe to call on every mount.
		const unsubscribe = initFirebase();
		// Once the first auth-state change fires, authResolved becomes true.
		const unsub = authUser.subscribe(() => {
			authResolved = true;
		});
		return () => {
			unsubscribe();
			unsub();
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
