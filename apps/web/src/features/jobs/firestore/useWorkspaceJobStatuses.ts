'use client';

import { useMemo } from 'react';
import { collection, limit, orderBy, query } from 'firebase/firestore';
import { db } from '@/lib/firebase';
import { useAuthedCollection } from '@/lib/firestore';
import type { FirestoreJobStatus } from '@/features/jobs/firestore/useJobStatus';

export function useWorkspaceJobStatuses(workspaceId: string, maxItems = 6) {
  const q = useMemo(
    () => (workspaceId
      ? query(
          collection(db, 'workspaces', workspaceId, 'jobs'),
          orderBy('updatedAt', 'desc'),
          limit(maxItems),
        )
      : null),
    [workspaceId, maxItems],
  );
  const { data } = useAuthedCollection<FirestoreJobStatus>(q, 'workspace_jobs');
  return { jobs: data ?? [] };
}
