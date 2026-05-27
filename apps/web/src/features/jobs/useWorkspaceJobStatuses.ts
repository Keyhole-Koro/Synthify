'use client';

import { useEffect, useState } from 'react';
import { collection, limit, onSnapshot, orderBy, query } from 'firebase/firestore';
import { db } from '@/lib/firebase';
import type { FirestoreJobStatus } from '@/features/jobs/useJobStatus';
import { type AppError } from '@/lib/errors';
import { toAppError } from '@/lib/error_normalize';

export function useWorkspaceJobStatuses(workspaceId: string, maxItems = 6) {
  const [jobs, setJobs] = useState<FirestoreJobStatus[]>([]);
  const [error, setError] = useState<AppError | null>(null);

  useEffect(() => {
    if (!workspaceId) return;

    const jobsQuery = query(
      collection(db, 'workspaces', workspaceId, 'jobs'),
      orderBy('updatedAt', 'desc'),
      limit(maxItems),
    );

    return onSnapshot(
      jobsQuery,
      (snapshot) => {
        const next = snapshot.docs.map((doc) => doc.data() as FirestoreJobStatus);
        setJobs(next);
        setError(null);
      },
      (err) => {
        setError(toAppError(err));
      },
    );
  }, [maxItems, workspaceId]);

  return { jobs, error };
}
