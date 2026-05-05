// Firebase Auth wrapper. Web uses the JS SDK directly; on Capacitor (iOS /
// Android) `@capacitor-firebase/authentication` is the native bridge — same
// public surface here so callers don't branch.
//
// Public-by-design config (apiKey etc.) lives in `import.meta.env.VITE_*`
// — safe to ship in the bundle.

import { initializeApp, type FirebaseApp } from 'firebase/app';
import {
  getAuth,
  signInWithPopup,
  signInWithEmailAndPassword,
  signInAnonymously as fbSignInAnon,
  GoogleAuthProvider,
  signOut as fbSignOut,
  onAuthStateChanged,
  type User,
  type Auth
} from 'firebase/auth';

let app: FirebaseApp | null = null;
let auth: Auth | null = null;

export function initFirebase(): Auth {
  if (auth) return auth;
  app = initializeApp({
    apiKey: import.meta.env.VITE_FIREBASE_API_KEY,
    authDomain: import.meta.env.VITE_FIREBASE_AUTH_DOMAIN,
    projectId: import.meta.env.VITE_FIREBASE_PROJECT_ID,
    appId: import.meta.env.VITE_FIREBASE_APP_ID
  });
  auth = getAuth(app);
  return auth;
}

export async function signInWithGoogle(): Promise<User> {
  const a = initFirebase();
  const cred = await signInWithPopup(a, new GoogleAuthProvider());
  return cred.user;
}

export async function signInWithEmail(email: string, password: string): Promise<User> {
  const a = initFirebase();
  const cred = await signInWithEmailAndPassword(a, email, password);
  return cred.user;
}

export async function signInAnonymously(): Promise<User> {
  const a = initFirebase();
  const cred = await fbSignInAnon(a);
  return cred.user;
}

export async function signOut(): Promise<void> {
  const a = initFirebase();
  await fbSignOut(a);
}

// getIdToken returns the current ID token. Pass forceRefresh=true to bypass
// the SDK's 5-minute cache (the WS reconnect path uses this).
export async function getIdToken(forceRefresh = false): Promise<string | null> {
  const a = initFirebase();
  const u = a.currentUser;
  if (!u) return null;
  return u.getIdToken(forceRefresh);
}

export { onAuthStateChanged };
export type { User };
