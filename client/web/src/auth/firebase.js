// Firebase Auth wrapper. Web uses the JS SDK directly; on Capacitor (iOS /
// Android) `@capacitor-firebase/authentication` is the native bridge — same
// public surface here so callers don't branch.
//
// Public-by-design config (apiKey etc.) lives in `import.meta.env.VITE_*`
// — safe to ship in the bundle.

import { initializeApp } from 'firebase/app';
import {
  getAuth,
  signInWithPopup,
  signInWithEmailAndPassword,
  signInAnonymously as fbSignInAnon,
  GoogleAuthProvider,
  signOut as fbSignOut,
  onAuthStateChanged
} from 'firebase/auth';

/** @typedef {import('firebase/app').FirebaseApp} FirebaseApp */
/** @typedef {import('firebase/auth').User} User */
/** @typedef {import('firebase/auth').Auth} Auth */

/** @type {FirebaseApp | null} */
let app = null;
/** @type {Auth | null} */
let auth = null;

/** @returns {Auth} */
export function initFirebase() {
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

/** @returns {Promise<User>} */
export async function signInWithGoogle() {
  const a = initFirebase();
  const cred = await signInWithPopup(a, new GoogleAuthProvider());
  return cred.user;
}

/**
 * @param {string} email
 * @param {string} password
 * @returns {Promise<User>}
 */
export async function signInWithEmail(email, password) {
  const a = initFirebase();
  const cred = await signInWithEmailAndPassword(a, email, password);
  return cred.user;
}

/** @returns {Promise<User>} */
export async function signInAnonymously() {
  const a = initFirebase();
  const cred = await fbSignInAnon(a);
  return cred.user;
}

/** @returns {Promise<void>} */
export async function signOut() {
  const a = initFirebase();
  await fbSignOut(a);
}

// getIdToken returns the current ID token. Pass forceRefresh=true to bypass
// the SDK's 5-minute cache (the WS reconnect path uses this).
/**
 * @param {boolean} [forceRefresh]
 * @returns {Promise<string | null>}
 */
export async function getIdToken(forceRefresh = false) {
  const a = initFirebase();
  const u = a.currentUser;
  if (!u) return null;
  return u.getIdToken(forceRefresh);
}

export { onAuthStateChanged };
