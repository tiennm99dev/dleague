// Rune-backed singleton auth state. Imported as `auth` everywhere — Svelte
// 5 picks up reactivity through `$state`. Initial load wires the
// onAuthStateChanged listener, so reactive consumers get repaint when the
// SDK rehydrates the user from persistence.

import { initFirebase, onAuthStateChanged, signInWithGoogle, signInWithEmail, signInAnonymously, signOut, getIdToken } from './firebase';

/** @typedef {import('./firebase').User} User */

class AuthState {
  user = $state(/** @type {User | null} */ (null));
  loading = $state(true);
  error = $state(/** @type {string | null} */ (null));

  constructor() {
    const a = initFirebase();
    onAuthStateChanged(a, (u) => {
      this.user = u;
      this.loading = false;
    });
  }

  async signInWithGoogle() {
    this.error = null;
    try {
      this.user = await signInWithGoogle();
    } catch (e) {
      this.error = /** @type {Error} */ (e).message;
    }
  }

  /**
   * @param {string} email
   * @param {string} password
   */
  async signInWithEmail(email, password) {
    this.error = null;
    try {
      this.user = await signInWithEmail(email, password);
    } catch (e) {
      this.error = /** @type {Error} */ (e).message;
    }
  }

  async signInAnonymously() {
    this.error = null;
    try {
      this.user = await signInAnonymously();
    } catch (e) {
      this.error = /** @type {Error} */ (e).message;
    }
  }

  async signOut() {
    await signOut();
    this.user = null;
  }

  /**
   * @param {boolean} [forceRefresh]
   * @returns {Promise<string | null>}
   */
  async getIdToken(forceRefresh = false) {
    return getIdToken(forceRefresh);
  }
}

export const auth = new AuthState();
