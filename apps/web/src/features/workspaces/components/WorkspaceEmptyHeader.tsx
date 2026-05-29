import React from 'react';
import { InlineError } from '@/components/error/InlineError';

interface WorkspaceEmptyHeaderProps {
  workspaceName: string;
  editingName: boolean;
  draftName: string;
  savingName: boolean;
  nameError: string | null;
  onDraftNameChange: (value: string) => void;
  onStartRename: () => void;
  onCommitName: () => void | Promise<void>;
  onCancelRename: () => void;
}

// WorkspaceEmptyHeader is the "Workspace" eyebrow + inline rename form that
// appears when a workspace card has no tree yet. The populated-mode header
// lives in WorkspaceHeader.tsx.
export function WorkspaceEmptyHeader({
  workspaceName,
  editingName,
  draftName,
  savingName,
  nameError,
  onDraftNameChange,
  onStartRename,
  onCommitName,
  onCancelRename,
}: WorkspaceEmptyHeaderProps) {
  return (
    <div>
      <p className="text-[10px] font-semibold uppercase tracking-[0.18em] text-indigo-400/80">Workspace</p>
      {editingName ? (
        <form
          className="mt-1 flex items-center gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            void onCommitName();
          }}
        >
          <input
            value={draftName}
            maxLength={64}
            disabled={savingName}
            onChange={(e) => onDraftNameChange(e.target.value)}
            onBlur={() => void onCommitName()}
            autoFocus
            className="min-w-0 flex-1 rounded-md border border-indigo-200 bg-white px-2 py-1 text-lg font-semibold tracking-tight text-stone-800 outline-none focus:border-indigo-400 focus:ring-2 focus:ring-indigo-100 disabled:opacity-60"
          />
          <button
            type="button"
            disabled={savingName}
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
          className="mt-0.5 block max-w-full truncate text-left text-lg font-semibold tracking-tight text-stone-800 hover:text-indigo-500"
        >
          {workspaceName}
        </button>
      )}
      {nameError && <InlineError message={nameError} className="mt-1" />}
    </div>
  );
}
