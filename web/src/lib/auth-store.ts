// Svelte store tracking the current Firebase auth user.
// Kept minimal: a single writable<User|null> with a helper to get the current
// ID token. WS module subscribes to this to drive connect/disconnect lifecycle.
import { writable, get } from 'svelte/store';
import type { User } from 'firebase/auth';

/** Reactive store: null = signed out, User = authenticated. */
export const authUser = writable<User | null>(null);

/**
 * setAuthUser is called by firebase.ts inside onAuthStateChanged.
 * Exported so firebase.ts can push updates without circular deps.
 */
export function setAuthUser(user: User | null): void {
	authUser.set(user);
}

/**
 * idToken fetches a fresh ID token for the currently signed-in user.
 * Throws if no user is signed in.
 * Firebase caches the token and only hits the network when <5 min to expiry.
 */
export async function idToken(force = false): Promise<string> {
	const user = get(authUser);
	if (!user) throw new Error('idToken: no user signed in');
	return user.getIdToken(force);
}
