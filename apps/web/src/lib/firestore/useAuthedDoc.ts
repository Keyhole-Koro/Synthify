'use client';

import { useEffect, useState } from 'react';
import { onSnapshot, type DocumentReference } from 'firebase/firestore';
import { onAuthStateChanged } from 'firebase/auth';
import { auth } from '@/lib/firebase';
import { type AppError } from '@/lib/errors';
import { classifyFirestoreSnapshotError } from './snapshotError';

export interface UseAuthedDocResult<T> {
  data: T | null;
  error: AppError | null;
}

// ref は呼び出し側で useMemo して渡す前提。auth uid が確定するまで subscribe
// しない / uid が消えたら detach して結果も null に倒す、というのが唯一の責務。
export function useAuthedDoc<T>(ref: DocumentReference | null): UseAuthedDocResult<T> {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<AppError | null>(null);
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
      (err) => {
        const classified = classifyFirestoreSnapshotError(err);
        if (classified.kind === 'transient') return;
        setError(classified.error);
      },
    );
  }, [uid, ref]);

  return { data: uid && ref ? data : null, error };
}
