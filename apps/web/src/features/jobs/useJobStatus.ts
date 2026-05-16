'use client';

import { useEffect, useState } from 'react';
import { doc, onSnapshot } from 'firebase/firestore';
import { db } from '@/lib/firebase';

export type JobStatus = {
  jobId: string;
  jobType: string;
  documentId: string;
  workspaceId: string;
  treeId: string;
  status: 'queued' | 'running' | 'succeeded' | 'failed';
  currentStage: string;
  progress?: number;
  message?: string;
  errorMessage: string;
  createdAt?: string;
  startedAt?: string;
  updatedAt: string;
  completedAt?: string;
};

export function useJobStatus(workspaceId: string, jobId: string | null) {
  const [status, setStatus] = useState<JobStatus | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!workspaceId || !jobId) return;

    const ref = doc(db, 'workspaces', workspaceId, 'jobs', jobId);
    return onSnapshot(
      ref,
      (snapshot) => {
        if (!snapshot.exists()) return;
        setStatus(snapshot.data() as JobStatus);
        setError(null);
      },
      (err) => {
        setError(err.message);
      },
    );
  }, [workspaceId, jobId]);

  return { status, error };
}
