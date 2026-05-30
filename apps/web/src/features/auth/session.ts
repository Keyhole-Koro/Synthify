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
import { log } from '@/lib/observability/log';

export type AuthUser = {
  id: string;
  email: string;
  name: string;
  accountId?: string;
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

// onAuthStateChanged が走るまで Firebase のセッションが存在するかは分からないが、
// 「直前のロードで authed だったか」は同期的に判定したい場面がある (例: SSR-like
// 初期描画で anonymous default を出すか、loading を待つかの判断)。Firebase の
// IndexedDB を覗くより、ログイン/ログアウトに同期して localStorage のマーカーを
// 更新する方が単純で確実なため、このフラグを用意している。
const SESSION_HINT_KEY = 'synthify:has_session';

export function hasPersistedSessionHint(): boolean {
  if (typeof window === 'undefined') return false;
  return window.localStorage.getItem(SESSION_HINT_KEY) === '1';
}

function writeSessionHint(present: boolean) {
  if (typeof window === 'undefined') return;
  if (present) {
    window.localStorage.setItem(SESSION_HINT_KEY, '1');
  } else {
    window.localStorage.removeItem(SESSION_HINT_KEY);
  }
}

export function subscribeAuthUser(callback: (user: AuthUser | null) => void): Unsubscribe {
  return onAuthStateChanged(auth, (user) => {
    writeSessionHint(user !== null);
    callback(fromFirebaseUser(user));
  });
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
    log.warn('getAuthHeaders called with no currentUser', { source: 'auth_headers' });
    return {};
  }
  const token = await auth.currentUser.getIdToken();
  return { Authorization: `Bearer ${token}` };
}
