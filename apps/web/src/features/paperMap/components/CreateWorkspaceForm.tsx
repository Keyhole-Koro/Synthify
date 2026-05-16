import React from 'react';

interface CreateWorkspaceFormProps {
  showCreateForm: boolean;
  newName: string;
  creating: boolean;
  loading: boolean;
  inputRef: React.RefObject<HTMLInputElement | null>;
  onSubmit: (e: React.FormEvent) => void;
  onToggleForm: (show: boolean) => void;
  onNameChange: (name: string) => void;
}

export function CreateWorkspaceForm({
  showCreateForm,
  newName,
  creating,
  loading,
  inputRef,
  onSubmit,
  onToggleForm,
  onNameChange,
}: CreateWorkspaceFormProps) {
  function stop(e: React.PointerEvent) { e.stopPropagation(); }

  if (showCreateForm) {
    return (
      <form onSubmit={onSubmit} className="flex flex-col gap-2 rounded-xl border border-indigo-100 bg-indigo-50/50 p-3">
        <p className="text-[10px] font-semibold uppercase tracking-[0.15em] text-indigo-400">新しいワークスペース</p>
        <input
          ref={inputRef}
          type="text"
          value={newName}
          onChange={(e) => onNameChange(e.target.value)}
          placeholder="ワークスペース名"
          maxLength={64}
          disabled={creating}
          onPointerDown={stop} onPointerUp={stop}
          className="w-full rounded-lg border border-indigo-200 bg-white px-3 py-2 text-sm text-stone-800 placeholder-stone-300 outline-none focus:border-indigo-400 focus:ring-2 focus:ring-indigo-100 disabled:opacity-60"
        />
        <div className="flex gap-2">
          <button
            type="submit"
            disabled={creating || !newName.trim()}
            onPointerDown={stop} onPointerUp={stop}
            className="flex-1 rounded-lg bg-indigo-500 py-2 text-xs font-semibold text-white transition-colors hover:bg-indigo-600 disabled:cursor-wait disabled:opacity-60"
          >
            {creating ? '作成中…' : '作成'}
          </button>
          <button
            type="button"
            disabled={creating}
            onPointerDown={stop} onPointerUp={stop}
            onClick={() => onToggleForm(false)}
            className="rounded-lg border border-stone-200 bg-white px-3 py-2 text-xs font-medium text-stone-500 transition-colors hover:bg-stone-50 disabled:opacity-60"
          >
            キャンセル
          </button>
        </div>
      </form>
    );
  }

  return (
    <button
      type="button"
      disabled={loading}
      onPointerDown={stop} onPointerUp={stop}
      onClick={() => onToggleForm(true)}
      className="flex w-full items-center justify-center gap-2 rounded-lg border border-dashed border-stone-200 py-2.5 text-xs font-medium text-stone-400 transition-colors hover:border-indigo-300 hover:text-indigo-500 disabled:cursor-wait disabled:opacity-60"
    >
      <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
      </svg>
      新しいワークスペースを作る
    </button>
  );
}
