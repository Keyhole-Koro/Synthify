import React, { useEffect, useRef, useState } from 'react';
import { type Workspace } from '@/features/workspaces/api';
import { WorkspaceItemList } from './components/WorkspaceItemList';
import { CreateWorkspaceForm } from './components/CreateWorkspaceForm';
import { WorkspaceError } from './components/WorkspaceError';

interface Props {
  workspaces: Workspace[];
  loading: boolean;
  error?: Error | null;
  onOpenWorkspace: (workspaceId: string) => void;
  onCreateWorkspace: (name: string) => Promise<void>;
  onLogout: () => void;
}

export function WorkspaceListContent({ workspaces, loading, error, onOpenWorkspace, onCreateWorkspace, onLogout }: Props) {
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [newName, setNewName] = useState('');
  const [creating, setCreating] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (showCreateForm) setTimeout(() => inputRef.current?.focus(), 0);
  }, [showCreateForm]);

  function stop(e: React.PointerEvent) { e.stopPropagation(); }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    const name = newName.trim();
    if (!name) return;
    setCreating(true);
    try {
      await onCreateWorkspace(name);
      setShowCreateForm(false);
      setNewName('');
    } finally {
      setCreating(false);
    }
  }

  if (error) {
    return <WorkspaceError onLogout={onLogout} />;
  }

  return (
    <div className="flex flex-col gap-3 pt-1">
      <WorkspaceItemList
        workspaces={workspaces}
        onOpenWorkspace={onOpenWorkspace}
      />

      <CreateWorkspaceForm
        showCreateForm={showCreateForm}
        newName={newName}
        creating={creating}
        loading={loading}
        inputRef={inputRef}
        onSubmit={handleSubmit}
        onToggleForm={setShowCreateForm}
        onNameChange={setNewName}
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
