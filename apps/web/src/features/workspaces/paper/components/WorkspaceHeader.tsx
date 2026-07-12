import React from 'react';

interface WorkspaceHeaderProps {
  workspaceName: string;
  draftName: string;
  editingName: boolean;
  savingName: boolean;
  childItemsCount: number;
  isRunning: boolean;
  isFailed: boolean;
  jobProgress?: number;
  isJustCompleted: boolean;
  onStartRename: () => void;
  onDraftNameChange: (name: string) => void;
  onCommitName: () => void;
  onCancelRename: () => void;
  canEdit?: boolean;
}

export function WorkspaceHeader({
  workspaceName,
  draftName,
  editingName,
  savingName,
  childItemsCount,
  isRunning,
  isFailed,
  jobProgress,
  isJustCompleted,
  onStartRename,
  onDraftNameChange,
  onCommitName,
  onCancelRename,
  canEdit = true,
}: WorkspaceHeaderProps) {
  return (
    <div
      className="flex w-full items-center justify-between gap-3 px-5 pt-4 pb-3 text-left"
    >
      <div className="min-w-0">
        <p className="text-[10px] font-semibold uppercase tracking-[0.18em] text-indigo-400/80">Workspace</p>
        {editingName ? (
          <form
            className="mt-0.5 flex min-w-0 items-center gap-2"
            onSubmit={(e) => {
              e.preventDefault();
              onCommitName();
            }}
          >
            <input
              value={draftName}
              maxLength={64}
              disabled={savingName}
              onChange={(e) => onDraftNameChange(e.target.value)}
              onBlur={onCommitName}
              autoFocus
              className="min-w-0 flex-1 rounded-md border border-indigo-200 bg-white px-2 py-1 text-lg font-semibold tracking-tight text-stone-800 outline-none focus:border-indigo-400 focus:ring-2 focus:ring-indigo-100 disabled:opacity-60"
            />
            <button
              type="button"
              disabled={savingName}
              onPointerDown={(e) => e.preventDefault()}
              onMouseDown={(e) => e.preventDefault()}
              onClick={onCancelRename}
              className="rounded-md border border-stone-200 px-2 py-1 text-[11px] text-stone-500 disabled:opacity-60"
            >
              取消
            </button>
          </form>
        ) : (
          <button
            type="button"
            onClick={onStartRename}
            disabled={!canEdit}
            className="mt-0.5 block max-w-full truncate text-left text-lg font-semibold tracking-tight text-stone-800 enabled:hover:text-indigo-500"
          >
            {workspaceName}
          </button>
        )}
        <div className="mt-0.5 flex items-center gap-2">
          <span className="text-[11px] text-stone-400">
            {childItemsCount} {childItemsCount === 1 ? 'doc' : 'docs'}
          </span>
          {isRunning && (
            <span className="text-[11px] font-semibold text-indigo-500">
              {typeof jobProgress === 'number' ? `running ${jobProgress}%` : 'analyzing...'}
            </span>
          )}
          {isFailed && (
            <span className="text-[11px] font-semibold text-red-500">
              failed
            </span>
          )}
          {isJustCompleted && !isRunning && (
            <span className="text-[11px] font-semibold text-emerald-500">just completed</span>
          )}
        </div>
      </div>
    </div>
  );
}
