'use client';

import { useEffect, useState } from 'react';
import { collection, limit, onSnapshot, orderBy, query } from 'firebase/firestore';
import { db } from '@/lib/firebase';
import type { JobStatus } from '@/features/jobs/useJobStatus';

export function useWorkspaceJobStatuses(workspaceId: string, maxItems = 6) {
  const [jobs, setJobs] = useState<JobStatus[]>([]);
  const [error, setError] = useState<string | null>(null);

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
        const next = snapshot.docs.map((doc) => doc.data() as JobStatus);
        setJobs(next);
        setError(null);
      },
      (err) => {
        setError(err.message);
      },
    );
  }, [maxItems, workspaceId]);

  return { jobs, error };
}
