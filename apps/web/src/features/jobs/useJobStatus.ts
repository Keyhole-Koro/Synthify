'use client';

import { useMemo } from 'react';
import { doc } from 'firebase/firestore';
import { db } from '@/lib/firebase';
import { useAuthedDoc } from '@/lib/firestore';
import type { FirestoreJobStatus } from '@/features/jobs/firestoreJobStatus.generated';

export type { FirestoreJobStatus } from '@/features/jobs/firestoreJobStatus.generated';

export function useJobStatus(workspaceId: string, jobId: string | null) {
  const ref = useMemo(
    () => (workspaceId && jobId ? doc(db, 'workspaces', workspaceId, 'jobs', jobId) : null),
    [workspaceId, jobId],
  );
  const { data, error } = useAuthedDoc<FirestoreJobStatus>(ref, 'job_status');
  return { status: data, error };
}
