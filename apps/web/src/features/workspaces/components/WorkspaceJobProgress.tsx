import React from 'react';

interface WorkspaceJobProgressProps {
  message?: string | null;
  progress?: number;
  isFailed: boolean;
}

export function WorkspaceJobProgress({
  message,
  progress,
  isFailed,
}: WorkspaceJobProgressProps) {
  return (
    <div className="mt-3 rounded-lg border border-stone-200 bg-white px-3 py-2">
      <div className="flex items-center justify-between gap-3">
        <span className="truncate text-[11px] font-medium text-stone-600">
          {message ?? '解析中'}
        </span>
        {typeof progress === 'number' && (
          <span className="text-[10px] font-semibold text-stone-400">{progress}%</span>
        )}
      </div>
      <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-stone-100">
        <div
          className={[
            'h-full rounded-full transition-all',
            isFailed ? 'bg-red-400' : 'bg-indigo-500',
          ].join(' ')}
          style={{ width: `${Math.max(4, Math.min(100, progress ?? 8))}%` }}
        />
      </div>
    </div>
  );
}
