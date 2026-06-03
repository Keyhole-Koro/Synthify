import React from 'react';
import { jobStatusLabel, jobStatusToneClass } from '@/features/jobs/contract/jobStatusContract';
import type { FirestoreJobStatus } from '@/features/jobs/useJobStatus';

interface WorkspaceJobListProps {
  workspaceJobs: FirestoreJobStatus[];
}

// WorkspaceJobList shows the last few jobs against this workspace. The
// currently active job already has its own progress bar
// (WorkspaceJobProgress), so this view is a quieter receipt-style list.
// documentId is intentionally not surfaced — it is a debug id, not a
// user-facing identifier.
export function WorkspaceJobList({ workspaceJobs }: WorkspaceJobListProps) {
  if (workspaceJobs.length === 0) {
    return null;
  }

  return (
    <div className="mt-4">
      <p className="mb-1.5 text-[10px] font-semibold uppercase tracking-[0.18em] text-stone-400">
        最近のジョブ
      </p>
      <ul className="divide-y divide-stone-100 overflow-hidden rounded-lg border border-stone-100 bg-white">
        {workspaceJobs.map((job) => (
          <li
            key={job.jobId}
            className="flex items-center gap-3 px-3 py-2 text-[11px]"
          >
            <StatusDot job={job} />
            <span className="flex-1 truncate text-stone-700">
              {job.message ?? jobStatusLabel(job)}
            </span>
            <span className="font-mono text-[10px] tabular-nums text-stone-400">
              {typeof job.progress === 'number' ? `${job.progress}%` : ''}
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}

function StatusDot({ job }: { job: FirestoreJobStatus }) {
  return <span className={`inline-block h-2 w-2 shrink-0 rounded-full ${jobStatusToneClass(job)}`} />;
}
