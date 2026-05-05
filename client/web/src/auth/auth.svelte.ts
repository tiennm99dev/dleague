// Rune-backed singleton auth state. Imported as `auth` everywhere — Svelte
// 5 picks up reactivity through `$state`. Initial load wires the
// onAuthStateChanged listener, so reactive consumers get repaint when the
// SDK rehydrates the user from persistence.

import { initFirebase, onAuthStateChanged, signInWithGoogle, signInWithEmail, signInAnonymously, signOut, getIdToken, type User } from './firebase';

class AuthState {
  user = $state<User | null>(null);
  loading = $state(true);
  error = $state<string | null>(null);

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
      this.error = (e as Error).message;
    }
  }

  async signInWithEmail(email: string, password: string) {
    this.error = null;
    try {
      this.user = await signInWithEmail(email, password);
    } catch (e) {
      this.error = (e as Error).message;
    }
  }

  async signInAnonymously() {
    this.error = null;
    try {
      this.user = await signInAnonymously();
    } catch (e) {
      this.error = (e as Error).message;
    }
  }

  async signOut() {
    await signOut();
    this.user = null;
  }

  async getIdToken(forceRefresh = false): Promise<string | null> {
    return getIdToken(forceRefresh);
  }
}

export const auth = new AuthState();
