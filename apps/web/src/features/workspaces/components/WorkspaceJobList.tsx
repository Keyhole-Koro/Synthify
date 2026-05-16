import React from 'react';
import type { JobStatus } from '@/features/jobs/useJobStatus';

interface WorkspaceJobListProps {
  workspaceJobs: JobStatus[];
  workspaceJobsError: string | null;
}

export function WorkspaceJobList({
  workspaceJobs,
  workspaceJobsError,
}: WorkspaceJobListProps) {
  if (workspaceJobs.length === 0 && !workspaceJobsError) {
    return null;
  }

  return (
    <div className="mt-4 rounded-xl border border-stone-200 bg-white/95 p-3 shadow-sm">
      <div className="mb-2 flex items-center justify-between">
        <p className="text-[10px] font-semibold uppercase tracking-[0.15em] text-stone-400">Recent Jobs</p>
        {workspaceJobsError && <span className="text-[10px] text-red-400">sync error</span>}
      </div>
      <div className="space-y-2">
        {workspaceJobs.map((job) => (
          <div key={job.jobId} className="rounded-lg border border-stone-100 bg-stone-50 px-3 py-2">
            <div className="flex items-center justify-between gap-3">
              <span className="truncate text-[11px] font-medium text-stone-700">
                {job.message || null}
              </span>
              <span className={jobStatusTone(job.status)}>{job.status}</span>
            </div>
            <div className="mt-1 flex items-center justify-between gap-3 text-[10px] text-stone-400">
              <span className="truncate">{job.documentId}</span>
              <span>{typeof job.progress === 'number' ? `${job.progress}%` : ''}</span>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function jobStatusTone(status: string) {
  switch (status) {
    case 'succeeded':
      return 'text-[10px] font-semibold uppercase tracking-wide text-emerald-600';
    case 'failed':
      return 'text-[10px] font-semibold uppercase tracking-wide text-red-500';
    case 'running':
      return 'text-[10px] font-semibold uppercase tracking-wide text-indigo-500';
    default:
      return 'text-[10px] font-semibold uppercase tracking-wide text-stone-400';
  }
}
