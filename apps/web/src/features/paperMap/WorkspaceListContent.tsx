import { useState } from 'react';
import {
  canSeedDevWorkspace,
  seedDevWorkspace,
  type Workspace,
} from '@/features/workspaces/api';
import { WorkspaceItemList } from './components/WorkspaceItemList';
import { CreateWorkspaceForm } from './components/CreateWorkspaceForm';
import { WorkspaceError } from './components/WorkspaceError';
import { type AppError } from '@/lib/errors';
import { toAppError } from '@/lib/error_normalize';

interface Props {
  workspaces: Workspace[];
  loading: boolean;
  error?: AppError | null;
  onOpenWorkspace: (workspaceId: string) => void;
  onCreateWorkspace: (name: string) => Promise<Workspace | void>;
  onRetry: () => void;
}

export function WorkspaceListContent({
  workspaces,
  loading,
  error,
  onOpenWorkspace,
  onCreateWorkspace,
  onRetry,
}: Props) {
  const [creating, setCreating] = useState(false);
  const [seeding, setSeeding] = useState(false);
  const [seedError, setSeedError] = useState<string | null>(null);

  async function handleCreate() {
    setCreating(true);
    try {
      await onCreateWorkspace('新規ワークスペース');
    } finally {
      setCreating(false);
    }
  }

  // dev seed: backend にデモ workspace を作らせ、できたら開く。
  async function handleSeedDevWorkspace() {
    setSeeding(true);
    setSeedError(null);
    try {
      const ws = await seedDevWorkspace();
      onOpenWorkspace(ws.workspaceId);
    } catch (err) {
      setSeedError(toAppError(err).message);
    } finally {
      setSeeding(false);
    }
  }

  if (error) {
    return <WorkspaceError error={error} onRetry={onRetry} />;
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

      {canSeedDevWorkspace() && (
        <div className="grid gap-1.5">
          <button
            type="button"
            disabled={loading || seeding}
            onPointerDown={(e) => e.stopPropagation()}
            onPointerUp={(e) => e.stopPropagation()}
            onClick={handleSeedDevWorkspace}
            className="flex w-full items-center justify-center gap-2 rounded-lg border border-emerald-100 bg-emerald-50 py-2.5 text-xs font-medium text-emerald-700 transition-colors hover:border-emerald-200 hover:bg-emerald-100 disabled:cursor-wait disabled:opacity-60"
          >
            <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 7h16M4 12h16M4 17h10" />
            </svg>
            {seeding ? 'seed 作成中...' : 'Dev seed workspace'}
          </button>
          {seedError && <p className="text-[11px] text-red-500">{seedError}</p>}
        </div>
      )}
    </div>
  );
}
