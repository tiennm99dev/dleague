// Firebase JS SDK initialisation and sign-in helpers.
// The web config (apiKey, projectId, etc.) is intentionally public — it only
// identifies the Firebase project, not server credentials. Restrict usage via
// Auth Domain allow-list in the Firebase console.
//
// Config priority: VITE_FIREBASE_* env vars (CI/prod) > firebase.config.json (local dev default).
import { initializeApp, type FirebaseApp } from 'firebase/app';
import {
	getAuth,
	connectAuthEmulator,
	onAuthStateChanged,
	signInWithEmailAndPassword,
	GoogleAuthProvider,
	signInWithPopup,
	signInAnonymously,
	signOut,
	type Auth
} from 'firebase/auth';

import jsonConfig from '../../firebase.config.json';
import { setAuthUser } from './auth-store';

const env = import.meta.env;

export const firebaseConfig = {
	apiKey:            env.VITE_FIREBASE_API_KEY            ?? jsonConfig.apiKey,
	authDomain:        env.VITE_FIREBASE_AUTH_DOMAIN        ?? jsonConfig.authDomain,
	projectId:         env.VITE_FIREBASE_PROJECT_ID         ?? jsonConfig.projectId,
	storageBucket:     env.VITE_FIREBASE_STORAGE_BUCKET     ?? jsonConfig.storageBucket,
	messagingSenderId: env.VITE_FIREBASE_MESSAGING_SENDER_ID ?? jsonConfig.messagingSenderId,
	appId:             env.VITE_FIREBASE_APP_ID             ?? jsonConfig.appId,
};

let app: FirebaseApp | null = null;
let auth: Auth | null = null;

/**
 * initFirebase initialises the Firebase app and subscribes to auth-state
 * changes. Safe to call multiple times; subsequent calls are no-ops.
 * Returns an unsubscribe function to be called on component destroy.
 */
export function initFirebase(): () => void {
	if (!app) {
		app = initializeApp(firebaseConfig);
		auth = getAuth(app);

		// Point at the local Auth emulator in dev so no real Firebase project is
		// needed. VITE_-prefixed env vars are inlined at build time by Vite.
		if (import.meta.env.DEV) {
			connectAuthEmulator(auth, 'http://127.0.0.1:9099', { disableWarnings: false });
		}
	}

	const unsubscribe = onAuthStateChanged(getAuth(app), (user) => {
		setAuthUser(user);
	});

	return unsubscribe;
}

/** Returns the Auth instance, throwing if initFirebase has not been called. */
function requireAuth(): Auth {
	if (!auth) throw new Error('Firebase not initialised — call initFirebase() first');
	return auth;
}

/** Sign in with email and password. */
export async function signInWithEmail(email: string, password: string): Promise<void> {
	await signInWithEmailAndPassword(requireAuth(), email, password);
}

/** Sign in with Google via popup (works with the Auth emulator too). */
export async function signInWithGoogle(): Promise<void> {
	const provider = new GoogleAuthProvider();
	await signInWithPopup(requireAuth(), provider);
}

/** Sign in anonymously — rate-limited by Firebase; excluded from leaderboards. */
export async function signInAnonymous(): Promise<void> {
	await signInAnonymously(requireAuth());
}

/** Sign out and clear auth state. */
export async function signOutNow(): Promise<void> {
	await signOut(requireAuth());
}
