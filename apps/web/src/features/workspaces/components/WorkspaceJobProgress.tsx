import React from 'react';

interface WorkspaceJobProgressProps {
  message?: string | null;
  progress?: number;
  isFailed: boolean;
}

// WorkspaceJobProgress is the in-place status bar for the currently active
// job on a workspace. It renders the stage label / percentage above a
// horizontal bar. When the worker has not yet reported a numeric progress
// (typeof progress !== 'number'), the bar fills the full width and shows an
// animated diagonal stripe so the UI does not look frozen during early
// stages — the stripe class lives in index.css.
export function WorkspaceJobProgress({
  message,
  progress,
  isFailed,
}: WorkspaceJobProgressProps) {
  const hasProgress = typeof progress === 'number';
  const clampedProgress = hasProgress ? Math.max(0, Math.min(100, progress)) : 100;
  const label = message ?? (isFailed ? '解析に失敗しました' : '解析中');

  return (
    <div className="mt-3">
      <div className="mb-1.5 flex items-baseline justify-between gap-3">
        <span
          className={[
            'truncate text-xs font-medium',
            isFailed ? 'text-red-600' : 'text-stone-700',
          ].join(' ')}
        >
          {label}
        </span>
        {hasProgress && !isFailed && (
          <span className="font-mono text-[10px] tabular-nums text-stone-400">
            {clampedProgress}%
          </span>
        )}
      </div>
      <div className="h-1.5 overflow-hidden rounded-full bg-stone-100">
        <div
          className={[
            'h-full rounded-full transition-[width] duration-500 ease-out',
            isFailed
              ? 'bg-red-500'
              : hasProgress
                ? 'bg-indigo-500'
                : 'progress-stripe-indeterminate bg-indigo-400',
          ].join(' ')}
          style={{ width: `${clampedProgress}%` }}
        />
      </div>
    </div>
  );
}
