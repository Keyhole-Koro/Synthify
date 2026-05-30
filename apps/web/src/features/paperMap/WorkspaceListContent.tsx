import { useState } from 'react';
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

  async function handleCreate() {
    setCreating(true);
    try {
      await onCreateWorkspace('新規ワークスペース');
    } finally {
      setCreating(false);
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
    </div>
  );
}
