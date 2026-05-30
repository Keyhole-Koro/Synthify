'use client';

import { useEffect, useState } from 'react';
import { onSnapshot, type Query } from 'firebase/firestore';
import { onAuthStateChanged } from 'firebase/auth';
import { auth } from '@/lib/firebase';
import { type AppError } from '@/lib/errors';
import { classifyFirestoreSnapshotError, reportSnapshotError } from './snapshotError';

export interface UseAuthedCollectionResult<T> {
  data: T[] | null;
  error: AppError | null;
}

// query は呼び出し側で useMemo して渡す前提。useAuthedDoc と同じ規約。
// label は観測用の任意ラベル（どのクエリで失敗したか NR で分かるように）。
export function useAuthedCollection<T>(q: Query | null, label?: string): UseAuthedCollectionResult<T> {
  const [data, setData] = useState<T[] | null>(null);
  const [error, setError] = useState<AppError | null>(null);
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
      (err) => {
        const classified = classifyFirestoreSnapshotError(err);
        reportSnapshotError(err, classified, { label });
        if (classified.kind === 'transient') return;
        setError(classified.error);
      },
    );
  }, [uid, q, label]);

  return { data: uid && q ? data : null, error };
}
