import { writable } from 'svelte/store';

export type AuthError = {
	kind: 'no_token' | 'auth_failed' | 'connection_failed';
	message: string;
} | null;

export const authError = writable<AuthError>(null);
