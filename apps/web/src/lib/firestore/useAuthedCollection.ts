import { useEffect, useState } from 'react';
import { onSnapshot, type Query, type FirestoreError } from 'firebase/firestore';
import { onAuthStateChanged } from 'firebase/auth';
import { auth } from '@/lib/firebase';
import type { UserFacingError } from '@/lib/errors';
import { handleSnapshotError } from './snapshotError';

/**
 * Result returned by the useAuthedCollection hook.
 *
 * @typeParam T - Expected item document shape.
 */
export interface UseAuthedCollectionResult<T> {
  /** Array of documents if query succeeded, or null if loading, unauthenticated, or empty ref. */
  data: T[] | null;
  /** Subscription error to show the user. Transient errors (connection drops, sign-out) stay null. */
  error: UserFacingError | null;
}

/**
 * Subscribes to real-time updates for a Firestore Query, gating subscription on an authenticated Firebase user session.
 *
 * @typeParam T - Document item type.
 * @param q - Firestore Query (should be memoized by caller).
 * @param label - Optional label for observability logging.
 * @returns Result containing the document array and any Firestore error.
 */
export function useAuthedCollection<T>(q: Query | null, label?: string): UseAuthedCollectionResult<T> {
  const [data, setData] = useState<T[] | null>(null);
  const [error, setError] = useState<UserFacingError | null>(null);
  const [uid, setUid] = useState<string | null>(auth.currentUser?.uid ?? null);

  useEffect(() => onAuthStateChanged(auth, (u) => setUid(u?.uid ?? null)), []);

  useEffect(() => {
    if (!uid || !q) return;
    return onSnapshot(
      q,
      (snapshot) => {
        setData(snapshot.docs.map((d) => d.data() as T));
        setError(null);
      },
      (err: FirestoreError) => {
        const shown = handleSnapshotError(err, label ?? 'collection');
        if (shown) setError(shown);
      },
    );
  }, [uid, q, label]);

  return { data: uid && q ? data : null, error };
}
