'use client';

import { useEffect, useState } from 'react';
import { collection, limit, onSnapshot, orderBy, query } from 'firebase/firestore';
import { onAuthStateChanged } from 'firebase/auth';
import { auth, db } from '@/lib/firebase';
import type { FirestoreJobStatus } from '@/features/jobs/useJobStatus';

export function useWorkspaceJobStatuses(workspaceId: string, maxItems = 6) {
  const [jobs, setJobs] = useState<FirestoreJobStatus[]>([]);
  const [authedUid, setAuthedUid] = useState<string | null>(auth.currentUser?.uid ?? null);

  useEffect(() => {
    return onAuthStateChanged(auth, (u) => setAuthedUid(u?.uid ?? null));
  }, []);

  useEffect(() => {
    if (!workspaceId || !authedUid) return;

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
      },
      (err) => {
        console.error('workspace job snapshot failed:', err);
      },
    );
  }, [maxItems, workspaceId, authedUid]);

  return { jobs };
}
