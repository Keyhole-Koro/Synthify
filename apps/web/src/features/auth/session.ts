'use client';

import {
  GoogleAuthProvider,
  onAuthStateChanged,
  signInWithPopup,
  signOut,
  type Unsubscribe,
  type User,
} from 'firebase/auth';
import { auth } from '@/lib/firebase';

export type AuthUser = {
  id: string;
  email: string;
  name: string;
};

function fromFirebaseUser(user: User | null): AuthUser | null {
  if (!user) return null;
  return {
    id: user.uid,
    email: user.email ?? '',
    name: user.displayName ?? user.email ?? 'Anonymous User',
  };
}

export function getInitialAuthUser(): AuthUser | null {
  return fromFirebaseUser(auth.currentUser);
}

export function subscribeAuthUser(callback: (user: AuthUser | null) => void): Unsubscribe {
  return onAuthStateChanged(auth, (user) => callback(fromFirebaseUser(user)));
}

export async function signInWithGoogleSession(): Promise<void> {
  const provider = new GoogleAuthProvider();
  await signInWithPopup(auth, provider);
}

export async function signOutSession(): Promise<void> {
  await signOut(auth);
}

export async function getAuthHeaders(): Promise<Record<string, string>> {
  if (!auth.currentUser) {
    console.warn('getAuthHeaders: no currentUser');
    return {};
  }
  const token = await auth.currentUser.getIdToken(true);
  return { Authorization: `Bearer ${token}` };
}
