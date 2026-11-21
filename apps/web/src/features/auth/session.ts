
import {
  GoogleAuthProvider,
  onAuthStateChanged,
  signInWithPopup,
  signOut,
  type Unsubscribe,
  type User,
} from 'firebase/auth';
import { auth } from '@/lib/firebase';
import { log } from '@/lib/observability/log';
import { ok, err, type Result } from '@/shared/result';
import { toAuthFailure, type AuthFailure } from './authError';

/**
 * Normalized user profile stored in application state.
 *
 * @property id Firebase UID used as the stable user key.
 * @property email Primary email address, empty when Firebase does not provide one.
 * @property name Display name, falling back to email and then `Anonymous User`.
 * @property accountId Optional billing/account identifier loaded by the API layer.
 */
export type AuthUser = {
  id: string;
  email: string;
  name: string;
  accountId?: string;
};

/**
 * Maps a Firebase User instance to the application AuthUser shape.
 *
 * @param user - Firebase user or null.
 * @returns Mapped AuthUser or null.
 */
function fromFirebaseUser(user: User | null): AuthUser | null {
  if (!user) return null;
  return {
    id: user.uid,
    email: user.email ?? '',
    name: user.displayName ?? user.email ?? 'Anonymous User',
  };
}

/**
 * Returns the currently signed-in Firebase user synchronously if already initialized.
 *
 * @returns Current AuthUser or null. This does not wait for Firebase Auth initialization;
 *   callers needing eventual state should use subscribeAuthUser.
 */
export function getInitialAuthUser(): AuthUser | null {
  return fromFirebaseUser(auth.currentUser);
}

// Key for persisting a fast synchronous session hint in localStorage.
// Allows initial UI rendering to know whether an authenticated session existed on last load.
const SESSION_HINT_KEY = 'synthify:has_session';

/**
 * Checks whether an active session was previously recorded in local storage.
 *
 * @returns True if the browser contains the local session hint; always false during SSR
 *   or when no authenticated session was recorded.
 */
export function hasPersistedSessionHint(): boolean {
  if (typeof window === 'undefined') return false;
  return window.localStorage.getItem(SESSION_HINT_KEY) === '1';
}

/**
 * Updates or removes the session hint in local storage.
 *
 * @param present - Whether an authenticated user is present.
 */
function writeSessionHint(present: boolean) {
  if (typeof window === 'undefined') return;
  if (present) {
    window.localStorage.setItem(SESSION_HINT_KEY, '1');
  } else {
    window.localStorage.removeItem(SESSION_HINT_KEY);
  }
}

/**
 * Subscribes to Firebase Auth state changes, updating localStorage session hints on change.
 *
 * @param callback - Listener called with the updated AuthUser or null.
 * @returns Unsubscribe function.
 */
export function subscribeAuthUser(callback: (user: AuthUser | null) => void): Unsubscribe {
  return onAuthStateChanged(auth, (user) => {
    writeSessionHint(user !== null);
    callback(fromFirebaseUser(user));
  });
}

/**
 * Initiates Google OAuth popup sign-in, returning a Result with AuthFailure on error.
 *
 * @returns A successful empty Result after Firebase resolves the popup, or an AuthFailure
 *   Result when the user cancels or Firebase rejects the sign-in.
 */
export async function signInWithGoogleSession(): Promise<Result<void, AuthFailure>> {
  try {
    const provider = new GoogleAuthProvider();
    await signInWithPopup(auth, provider);
    return ok(undefined);
  } catch (caught) {
    return err(toAuthFailure(caught));
  }
}

/**
 * Signs the user out of Firebase Auth and clears Firebase's active user state.
 *
 * @returns Promise fulfilled after Firebase has completed sign-out.
 */
export async function signOutSession(): Promise<void> {
  await signOut(auth);
}

/**
 * Obtains an Authorization header containing the current user's Firebase ID token.
 *
 * @returns Authorization header map with a fresh Bearer token, or an empty object when no
 *   user is currently signed in. Token acquisition errors are propagated to the caller.
 */
export async function getAuthHeaders(): Promise<Record<string, string>> {
  if (!auth.currentUser) {
    log.warn('getAuthHeaders called with no currentUser', { source: 'auth_headers' });
    return {};
  }
  const token = await auth.currentUser.getIdToken();
  return { Authorization: `Bearer ${token}` };
}
