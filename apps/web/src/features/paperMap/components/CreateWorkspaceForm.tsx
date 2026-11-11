import React from 'react';

/**
 * Props for the {@link CreateWorkspaceForm} button component.
 */
interface CreateWorkspaceFormProps {
  /** Indicates if a workspace creation request is currently in progress. */
  creating: boolean;
  /** Indicates if initial or workspace listing data is currently loading. */
  loading: boolean;
  /** Callback invoked when the user triggers workspace creation. */
  onCreate: () => void;
}

/**
 * Renders a dashed action button allowing users to quickly initialize
 * a new workspace in the paper canvas navigation view.
 *
 * @param props - Component properties.
 * @returns The rendered workspace creation trigger button.
 */
export function CreateWorkspaceForm({
  creating,
  loading,
  onCreate,
}: CreateWorkspaceFormProps) {
  function stop(e: React.PointerEvent) { e.stopPropagation(); }

  return (
    <button
      type="button"
      disabled={loading || creating}
      onPointerDown={stop} onPointerUp={stop}
      onClick={onCreate}
      className="flex w-full items-center justify-center gap-2 rounded-lg border border-dashed border-stone-200 py-2.5 text-xs font-medium text-stone-400 transition-colors hover:border-indigo-300 hover:text-indigo-500 disabled:cursor-wait disabled:opacity-60"
    >
      <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
      </svg>
      {creating ? '作成中…' : '新規ワークスペース'}
    </button>
  );
}
