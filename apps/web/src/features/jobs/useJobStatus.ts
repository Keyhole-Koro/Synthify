'use client';

import { useEffect, useState } from 'react';
import { doc, onSnapshot } from 'firebase/firestore';
import { onAuthStateChanged } from 'firebase/auth';
import { auth, db } from '@/lib/firebase';
import type { FirestoreJobStatus } from '@/features/jobs/firestoreJobStatus.generated';
import { type AppError } from '@/lib/errors';
import { toAppError } from '@/lib/error_normalize';

export type { FirestoreJobStatus } from '@/features/jobs/firestoreJobStatus.generated';

export function useJobStatus(workspaceId: string, jobId: string | null) {
  const [status, setStatus] = useState<FirestoreJobStatus | null>(null);
  const [error, setError] = useState<AppError | null>(null);
  const [authedUid, setAuthedUid] = useState<string | null>(auth.currentUser?.uid ?? null);

  useEffect(() => {
    return onAuthStateChanged(auth, (u) => setAuthedUid(u?.uid ?? null));
  }, []);

  useEffect(() => {
    if (!workspaceId || !jobId || !authedUid) return;

    const ref = doc(db, 'workspaces', workspaceId, 'jobs', jobId);
    return onSnapshot(
      ref,
      (snapshot) => {
        if (!snapshot.exists()) return;
        setStatus(snapshot.data() as FirestoreJobStatus);
        setError(null);
      },
      (err) => {
        setError(toAppError(err));
      },
    );
  }, [workspaceId, jobId, authedUid]);

  return { status, error };
}
