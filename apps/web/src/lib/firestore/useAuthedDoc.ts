import { useEffect, useState } from 'react';
import { onSnapshot, type DocumentReference, type FirestoreError } from 'firebase/firestore';
import { onAuthStateChanged } from 'firebase/auth';
import { auth } from '@/lib/firebase';
import type { UserFacingError } from '@/lib/errors';
import { handleSnapshotError } from './snapshotError';

/**
 * Result returned by the useAuthedDoc hook.
 *
 * @typeParam T - Expected document data shape.
 */
export interface UseAuthedDocResult<T> {
  /** Document data if found, or null if loading, unauthenticated, or non-existent. */
  data: T | null;
  /** Subscription error to show the user. Transient errors (connection drops, sign-out) stay null. */
  error: UserFacingError | null;
}

/**
 * Subscribes to real-time updates for a single Firestore document reference,
 * gating subscription on an authenticated Firebase user session.
 *
 * @typeParam T - Document data type.
 * @param ref - Firestore DocumentReference (should be memoized by caller).
 * @param label - Optional label for observability logging.
 * @returns Result containing the document data and any Firestore error.
 */
export function useAuthedDoc<T>(ref: DocumentReference | null, label?: string): UseAuthedDocResult<T> {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<UserFacingError | null>(null);
  const [uid, setUid] = useState<string | null>(auth.currentUser?.uid ?? null);

  useEffect(() => onAuthStateChanged(auth, (u) => setUid(u?.uid ?? null)), []);

  useEffect(() => {
    if (!uid || !ref) return;
    return onSnapshot(
      ref,
      (snapshot) => {
        setData(snapshot.exists() ? (snapshot.data() as T) : null);
        setError(null);
      },
      (err: FirestoreError) => {
        const shown = handleSnapshotError(err, label ?? ref.path);
        if (shown) setError(shown);
      },
    );
  }, [uid, ref, label]);

  return { data: uid && ref ? data : null, error };
}
