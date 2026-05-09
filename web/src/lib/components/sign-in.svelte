<script lang="ts">
	// Sign-in component offering three Firebase auth providers.
	// Email/password form + Google popup + anonymous sign-in.
	import {
		signInWithEmail,
		signInWithGoogle,
		signInAnonymous
	} from '$lib/firebase';
	import AnonymousWarning from './anonymous-warning.svelte';

	let email = $state('');
	let password = $state('');
	let errorMsg = $state('');
	let loading = $state(false);

	// Map Firebase error codes to user-friendly messages.
	// NOTE: auth/wrong-password and auth/user-not-found intentionally map to the
	// same string — never reveal whether the account exists.
	function friendlyAuthError(err: unknown): string {
		const code = (err as { code?: string })?.code ?? '';
		console.error('Auth error:', code, err);
		switch (code) {
			case 'auth/wrong-password':
			case 'auth/user-not-found':
			case 'auth/invalid-credential':
				return 'Incorrect email or password.';
			case 'auth/invalid-email':
				return 'Invalid email address.';
			case 'auth/too-many-requests':
				return 'Too many attempts. Try again later.';
			case 'auth/network-request-failed':
				return 'Network error. Check your connection.';
			case 'auth/popup-closed-by-user':
				return 'Sign-in cancelled.';
			default:
				return 'Sign-in failed. Please try again.';
		}
	}

	async function handleEmail(e: SubmitEvent): Promise<void> {
		e.preventDefault();
		errorMsg = '';
		loading = true;
		try {
			await signInWithEmail(email, password);
		} catch (err) {
			errorMsg = friendlyAuthError(err);
		} finally {
			loading = false;
		}
	}

	async function handleGoogle(): Promise<void> {
		errorMsg = '';
		loading = true;
		try {
			await signInWithGoogle();
		} catch (err) {
			errorMsg = friendlyAuthError(err);
		} finally {
			loading = false;
		}
	}

	async function handleAnonymous(): Promise<void> {
		errorMsg = '';
		loading = true;
		try {
			await signInAnonymous();
		} catch (err) {
			errorMsg = friendlyAuthError(err);
		} finally {
			loading = false;
		}
	}
</script>

<div class="sign-in-container">
	<h1>dleague</h1>
	<p class="subtitle">daily word-game league</p>

	<form onsubmit={handleEmail} class="email-form">
		<label for="email">Email</label>
		<input
			id="email"
			type="email"
			bind:value={email}
			placeholder="you@example.com"
			required
			disabled={loading}
		/>
		<label for="password">Password</label>
		<input
			id="password"
			type="password"
			bind:value={password}
			placeholder="password"
			required
			disabled={loading}
		/>
		<button type="submit" disabled={loading}>
			{loading ? 'Signing in…' : 'Sign in with Email'}
		</button>
	</form>

	<div class="divider">or</div>

	<button onclick={handleGoogle} disabled={loading} class="btn-google">
		Sign in with Google
	</button>

	<AnonymousWarning inline />
	<button onclick={handleAnonymous} disabled={loading} class="btn-anon">
		Continue anonymously
	</button>

	{#if errorMsg}
		<p class="error" role="alert">{errorMsg}</p>
	{/if}
</div>

<style>
	.sign-in-container {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		min-height: 100vh;
		background: #1a1a2e;
		color: #e0e0ff;
		font-family: monospace;
		gap: 0.75rem;
		padding: 2rem;
	}

	h1 {
		font-size: 3rem;
		letter-spacing: 0.1em;
		margin: 0;
		color: #e0e0ff;
	}

	.subtitle {
		color: #8888cc;
		margin: 0 0 1rem;
	}

	.email-form {
		display: flex;
		flex-direction: column;
		gap: 0.4rem;
		width: 100%;
		max-width: 320px;
	}

	input {
		padding: 0.5rem 0.75rem;
		border-radius: 6px;
		border: 1px solid #4444cc;
		background: #0f0f1a;
		color: #e0e0ff;
		font-size: 1rem;
	}

	button {
		padding: 0.6rem 1.2rem;
		border-radius: 6px;
		border: none;
		cursor: pointer;
		font-family: monospace;
		font-size: 1rem;
		transition: opacity 0.15s;
		width: 100%;
		max-width: 320px;
	}

	button:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	button[type='submit'] {
		background: #4444cc;
		color: #fff;
		margin-top: 0.25rem;
	}

	.btn-google {
		background: #ffffff;
		color: #333;
	}

	.btn-anon {
		background: #2a2a4e;
		color: #8888cc;
		border: 1px solid #4444cc;
	}

	.divider {
		color: #555580;
		font-size: 0.85rem;
	}

	.error {
		color: #ff6666;
		font-size: 0.9rem;
		max-width: 320px;
		text-align: center;
	}
</style>
