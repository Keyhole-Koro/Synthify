import React from 'react';
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
            <StatusDot status={job.status} />
            <span className="flex-1 truncate text-stone-700">
              {job.message ?? statusLabel(job.status)}
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

function StatusDot({ status }: { status: string }) {
  const tone = (() => {
    switch (status) {
      case 'succeeded':
        return 'bg-emerald-500';
      case 'failed':
        return 'bg-red-500';
      case 'running':
        return 'bg-indigo-500 animate-pulse';
      case 'queued':
        return 'bg-amber-400';
      default:
        return 'bg-stone-300';
    }
  })();
  return <span className={`inline-block h-2 w-2 shrink-0 rounded-full ${tone}`} />;
}

function statusLabel(status: string): string {
  switch (status) {
    case 'succeeded':
      return '完了';
    case 'failed':
      return '失敗';
    case 'running':
      return '実行中';
    case 'queued':
      return '待機中';
    default:
      return status;
  }
}
