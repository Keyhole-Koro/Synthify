/**
 * Client-side authentication helpers and authenticated fetch client using Firebase Web SDK.
 *
 * @module
 */

import {
  GoogleAuthProvider,
  onAuthStateChanged,
  signInWithEmailAndPassword,
  signInWithPopup,
  signOut,
  type Unsubscribe,
  type User,
} from 'firebase/auth';
import { getFirebaseAuth } from './firebase-client';

/**
 * Subscribes to Firebase user authentication state changes.
 *
 * @param callback Observer invoked whenever the current authenticated user changes.
 * @returns Teardown unsubscribe function.
 */
export function subscribeUser(callback: (user: User | null) => void): Unsubscribe {
  return onAuthStateChanged(getFirebaseAuth(), callback);
}

/**
 * Initiates interactive Google OAuth sign-in flow via popup.
 *
 * @returns Promise fulfilled after Firebase has established the user session.
 * @throws Propagates Firebase popup errors, including user cancellation and provider failures.
 */
export async function signInWithGoogle(): Promise<void> {
  await signInWithPopup(getFirebaseAuth(), new GoogleAuthProvider());
}

/**
 * Signs in using email and password credentials.
 *
 * @param email User account email.
 * @param password User account password.
 * @returns Promise fulfilled after Firebase accepts the credentials and establishes a session.
 * @throws Propagates Firebase credential, rate-limit, and network errors.
 */
export async function signInWithPassword(email: string, password: string): Promise<void> {
  await signInWithEmailAndPassword(getFirebaseAuth(), email, password);
}

/**
 * Signs out the currently authenticated user.
 *
 * @returns Promise fulfilled after Firebase clears the current session.
 * @throws Propagates Firebase sign-out failures.
 */
export async function signOutUser(): Promise<void> {
  await signOut(getFirebaseAuth());
}

/**
 * Performs an HTTP fetch attaching the current user's Firebase ID token as a Bearer authorization header.
 *
 * @param input The request URL or Request object.
 * @param init Optional fetch options.
 * @returns Promise resolving to the fetch Response.
 */
export async function authFetch(input: string, init: RequestInit = {}): Promise<Response> {
  const headers = new Headers(init.headers);
  const user = getFirebaseAuth().currentUser;
  if (user) {
    const token = await user.getIdToken();
    headers.set('Authorization', `Bearer ${token}`);
  }
  return fetch(input, { ...init, headers });
}
