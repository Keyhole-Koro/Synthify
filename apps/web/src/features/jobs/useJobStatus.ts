'use client';

import { useEffect, useState } from 'react';
import { doc, onSnapshot } from 'firebase/firestore';
import { db } from '@/lib/firebase';
import type { FirestoreJobStatus } from '@/features/jobs/firestoreJobStatus.generated';

export type { FirestoreJobStatus } from '@/features/jobs/firestoreJobStatus.generated';

export function useJobStatus(workspaceId: string, jobId: string | null) {
  const [status, setStatus] = useState<FirestoreJobStatus | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!workspaceId || !jobId) return;

    const ref = doc(db, 'workspaces', workspaceId, 'jobs', jobId);
    return onSnapshot(
      ref,
      (snapshot) => {
        if (!snapshot.exists()) return;
        setStatus(snapshot.data() as FirestoreJobStatus);
        setError(null);
      },
      (err) => {
        setError(err.message);
      },
    );
  }, [workspaceId, jobId]);

  return { status, error };
}
