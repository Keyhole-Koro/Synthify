import React from 'react';

interface WorkspaceHeaderProps {
  workspaceName: string;
  childItemsCount: number;
  isRunning: boolean;
  jobProgress?: number;
  isJustCompleted: boolean;
  isPinned: boolean;
  onTogglePinned: () => void;
}

export function WorkspaceHeader({
  workspaceName,
  childItemsCount,
  isRunning,
  jobProgress,
  isJustCompleted,
  isPinned,
  onTogglePinned,
}: WorkspaceHeaderProps) {
  return (
    <button
      type="button"
      onClick={onTogglePinned}
      className="flex w-full items-center justify-between gap-3 px-5 pt-4 pb-3 text-left"
    >
      <div className="min-w-0">
        <p className="text-[10px] font-semibold uppercase tracking-[0.18em] text-indigo-400/80">Workspace</p>
        <h2 className="mt-0.5 truncate text-lg font-semibold tracking-tight text-stone-800">{workspaceName}</h2>
        <div className="mt-0.5 flex items-center gap-2">
          <span className="text-[11px] text-stone-400">{childItemsCount} docs</span>
          {isRunning && typeof jobProgress === 'number' && (
            <span className="text-[11px] font-semibold text-indigo-500">
              running {jobProgress}%
            </span>
          )}
          {isJustCompleted && !isRunning && (
            <span className="text-[11px] font-semibold text-emerald-500">just completed</span>
          )}
        </div>
      </div>
      <div className="flex shrink-0 items-center gap-2">
        <div
          className={[
            'flex h-7 w-7 items-center justify-center rounded-md border transition-colors',
            isPinned
              ? 'border-indigo-300 bg-indigo-50 text-indigo-500'
              : 'border-stone-200 bg-white text-stone-400 hover:border-indigo-300 hover:text-indigo-500',
          ].join(' ')}
          title={isPinned ? '閉じる' : '詳細を固定表示'}
        >
          <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
          </svg>
        </div>
      </div>
    </button>
  );
}
