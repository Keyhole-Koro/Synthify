import React, { useState } from 'react';
import { type Workspace } from '@/features/workspaces/api';
import { WorkspaceItemList } from './components/WorkspaceItemList';
import { CreateWorkspaceForm } from './components/CreateWorkspaceForm';
import { WorkspaceError } from './components/WorkspaceError';
import { type AppError } from '@/lib/errors';

interface Props {
  workspaces: Workspace[];
  loading: boolean;
  error?: AppError | null;
  onOpenWorkspace: (workspaceId: string) => void;
  onCreateWorkspace: (name: string) => Promise<void>;
  onRetry: () => void;
  onLogout: () => void;
}

export function WorkspaceListContent({
  workspaces,
  loading,
  error,
  onOpenWorkspace,
  onCreateWorkspace,
  onRetry,
  onLogout,
}: Props) {
  const [creating, setCreating] = useState(false);

  function stop(e: React.PointerEvent) { e.stopPropagation(); }

  async function handleCreate() {
    setCreating(true);
    try {
      await onCreateWorkspace('新規ワークスペース');
    } finally {
      setCreating(false);
    }
  }

  if (error) {
    return <WorkspaceError error={error} onRetry={onRetry} onLogout={onLogout} />;
  }

  return (
    <div className="flex flex-col gap-3 pt-1">
      <WorkspaceItemList
        workspaces={workspaces}
        onOpenWorkspace={onOpenWorkspace}
      />

      <CreateWorkspaceForm
        creating={creating}
        loading={loading}
        onCreate={handleCreate}
      />

      {/* Logout */}
      <button
        type="button"
        onPointerDown={stop} onPointerUp={stop}
        onClick={onLogout}
        className="w-full rounded-lg border border-stone-200 py-2 text-xs font-medium text-stone-400 transition-colors hover:bg-stone-50 hover:text-stone-600"
      >
        ログアウト
      </button>
    </div>
  );
}
