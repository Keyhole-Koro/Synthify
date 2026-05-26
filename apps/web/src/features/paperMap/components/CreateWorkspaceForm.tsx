import React from 'react';

interface CreateWorkspaceFormProps {
  creating: boolean;
  loading: boolean;
  onCreate: () => void;
}

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
