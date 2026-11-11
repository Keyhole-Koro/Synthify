
import { useMemo } from 'react';
import { doc } from 'firebase/firestore';
import { db } from '@/lib/firebase';
import { useAuthedDoc } from '@/lib/firestore';
import type { FirestoreJobStatus } from '@/features/jobs/firestore/firestoreJobStatus.generated';

export type { FirestoreJobStatus } from '@/features/jobs/firestore/firestoreJobStatus.generated';

/**
 * Subscribes in real time to the Firestore status document of a specific background processing job.
 *
 * @param workspaceId - Workspace containing the job.
 * @param jobId - Job ULID identifier, or null.
 * @returns Status document data and snapshot error.
 */
export function useJobStatus(workspaceId: string, jobId: string | null) {
  const ref = useMemo(
    () => (workspaceId && jobId ? doc(db, 'workspaces', workspaceId, 'jobs', jobId) : null),
    [workspaceId, jobId],
  );
  const { data, error } = useAuthedDoc<FirestoreJobStatus>(ref, 'job_status');
  return { status: data, error };
}
