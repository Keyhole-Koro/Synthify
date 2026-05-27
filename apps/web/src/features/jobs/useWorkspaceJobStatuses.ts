'use client';

import { useEffect, useState } from 'react';
import { collection, limit, onSnapshot, orderBy, query } from 'firebase/firestore';
import { db } from '@/lib/firebase';
import type { FirestoreJobStatus } from '@/features/jobs/useJobStatus';

export function useWorkspaceJobStatuses(workspaceId: string, maxItems = 6) {
  const [jobs, setJobs] = useState<FirestoreJobStatus[]>([]);

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
      },
      (err) => {
        // Firestore onSnapshot は permission denied / 一時的なネットワーク断 /
        // quota 超過などで発火するが、いずれもユーザー側で取れるアクションが
        // ない (SDK が自動再接続する)。UI に出すと「sync error」が誤って
        // 操作を促してしまうため、観測用に console にだけ残す。
        console.error('workspace job snapshot failed:', err);
      },
    );
  }, [maxItems, workspaceId]);

  return { jobs };
}
