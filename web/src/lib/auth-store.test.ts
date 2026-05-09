// Tests for auth-store.ts: idToken(force) overload, null-user guard.
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { authUser, idToken } from './auth-store';
import type { User } from 'firebase/auth';

function makeUser(token = 'id-token-value'): User {
	return {
		getIdToken: vi.fn().mockResolvedValue(token)
	} as unknown as User;
}

describe('idToken', () => {
	beforeEach(() => {
		// Reset store to null before each test.
		authUser.set(null);
	});

	it('throws when no user is signed in', async () => {
		await expect(idToken()).rejects.toThrow('idToken: no user signed in');
	});

	it('defaults to force=false', async () => {
		const user = makeUser();
		authUser.set(user);

		await idToken();

		expect(user.getIdToken).toHaveBeenCalledWith(false);
	});

	it('passes force=true when explicitly requested', async () => {
		const user = makeUser();
		authUser.set(user);

		await idToken(true);

		expect(user.getIdToken).toHaveBeenCalledWith(true);
	});

	it('returns the token string from getIdToken', async () => {
		const user = makeUser('my-firebase-token');
		authUser.set(user);

		const result = await idToken();

		expect(result).toBe('my-firebase-token');
	});
});
