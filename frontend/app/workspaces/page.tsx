'use client';

import { useEffect, useRef, useState } from 'react';
import { useRouter } from 'next/navigation';
import { onAuthStateChanged, signOut, type User } from 'firebase/auth';
import { auth } from '@/lib/firebase';
import { listWorkspaces, createWorkspace, type Workspace } from '@/features/workspaces/api';

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('ja-JP', { year: 'numeric', month: 'short', day: 'numeric' });
}


function getUserInitial(user: User | null): string {
  const source = user?.displayName || user?.email || '';
  return source.trim().charAt(0).toUpperCase() || '?';
}

export default function WorkspacesPage() {
  const router = useRouter();
  const [user, setUser] = useState<User | null>(null);
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [showForm, setShowForm] = useState(false);
  const [newName, setNewName] = useState('');
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    return onAuthStateChanged(auth, async (u) => {
      if (!u) {
        router.replace('/');
        return;
      }
      setUser(u);
      try {
        const ws = await listWorkspaces();
        setWorkspaces(ws);
      } catch (err) {
        console.error(err);
      } finally {
        setLoading(false);
      }
    });
  }, [router]);

  useEffect(() => {
    if (showForm) {
      setTimeout(() => inputRef.current?.focus(), 0);
    }
  }, [showForm]);

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    const name = newName.trim();
    if (!name) return;
    setCreating(true);
    try {
      const ws = await createWorkspace(name);
      router.push(`/w/${ws.workspace_id}`);
    } catch (err) {
      console.error(err);
      alert('ワークスペースの作成に失敗しました。');
      setCreating(false);
    }
  }

  function handleCancel() {
    setShowForm(false);
    setNewName('');
  }

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-slate-50">
        <div className="h-6 w-6 animate-spin rounded-full border-2 border-indigo-600 border-t-transparent" />
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-slate-50">
      {/* Header */}
      <header className="border-b border-slate-200 bg-white px-6 py-4">
        <div className="mx-auto flex max-w-3xl items-center justify-between">
          <div className="flex items-center gap-2.5">
            <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-indigo-500">
              <svg className="h-3.5 w-3.5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 6.042A8.967 8.967 0 006 3.75c-1.052 0-2.062.18-3 .512v14.25A8.987 8.987 0 016 18c2.305 0 4.408.867 6 2.292m0-14.25a8.966 8.966 0 016-2.292c1.052 0 2.062.18 3 .512v14.25A8.987 8.987 0 0018 18a8.967 8.967 0 00-6 2.292m0-14.25v14.25" />
              </svg>
            </div>
            <span className="text-sm font-semibold tracking-tight text-stone-800">Synthify</span>
          </div>

          <div className="flex items-center gap-3">
            {user?.photoURL ? (
              <img
                src={user.photoURL}
                alt={user.displayName || user.email || 'Account avatar'}
                className="h-8 w-8 rounded-full border border-slate-200 object-cover"
              />
            ) : (
              <div className="flex h-8 w-8 items-center justify-center rounded-full bg-indigo-100 text-sm font-semibold text-indigo-700">
                {getUserInitial(user)}
              </div>
            )}
            <span className="text-sm text-slate-500">{user?.email}</span>
            <button
              onClick={() => signOut(auth)}
              className="text-xs text-slate-400 hover:text-slate-600 transition-colors"
            >
              ログアウト
            </button>
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-3xl px-6 py-10">
        {/* Title row */}
        <div className="mb-6 flex items-center justify-between">
          <h1 className="text-lg font-semibold text-slate-900">ワークスペース</h1>
          {!showForm && (
            <button
              onClick={() => setShowForm(true)}
              className="flex items-center gap-1.5 rounded-lg bg-indigo-600 px-3.5 py-2 text-sm font-semibold text-white hover:bg-indigo-700 transition-colors"
            >
              <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
              </svg>
              新規作成
            </button>
          )}
        </div>

        {/* Create form */}
        {showForm && (
          <form
            onSubmit={handleCreate}
            className="mb-4 rounded-xl border border-indigo-200 bg-white px-5 py-4 shadow-sm"
          >
            <p className="mb-3 text-sm font-medium text-slate-700">新しいワークスペース</p>
            <div className="flex gap-2">
              <input
                ref={inputRef}
                type="text"
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                placeholder="ワークスペース名"
                maxLength={64}
                disabled={creating}
                className="flex-1 rounded-lg border border-slate-200 px-3 py-2 text-sm text-slate-800 placeholder-slate-400 outline-none focus:border-indigo-400 focus:ring-2 focus:ring-indigo-100 disabled:opacity-60"
              />
              <button
                type="submit"
                disabled={creating || !newName.trim()}
                className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-700 disabled:opacity-50 transition-colors"
              >
                {creating ? '作成中…' : '作成'}
              </button>
              <button
                type="button"
                onClick={handleCancel}
                disabled={creating}
                className="rounded-lg border border-slate-200 px-3 py-2 text-sm text-slate-500 hover:text-slate-700 transition-colors"
              >
                キャンセル
              </button>
            </div>
          </form>
        )}

        {/* Workspace list */}
        {workspaces.length === 0 ? (
          <div className="rounded-xl border border-dashed border-slate-300 bg-white p-12 text-center">
            <div className="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-full bg-slate-100">
              <svg className="h-6 w-6 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M2.25 12.75V12A2.25 2.25 0 014.5 9.75h15A2.25 2.25 0 0121.75 12v.75m-8.69-6.44l-2.12-2.12a1.5 1.5 0 00-1.061-.44H4.5A2.25 2.25 0 002.25 6v12a2.25 2.25 0 002.25 2.25h15A2.25 2.25 0 0021.75 18V9a2.25 2.25 0 00-2.25-2.25h-5.379a1.5 1.5 0 01-1.06-.44z" />
              </svg>
            </div>
            <p className="text-sm font-medium text-slate-600">ワークスペースがありません</p>
            <p className="mt-1 text-xs text-slate-400">「新規作成」から最初のワークスペースを作りましょう</p>
          </div>
        ) : (
          <ul className="space-y-3">
            {workspaces.map((ws) => (
              <WorkspaceCard
                key={ws.workspace_id}
                workspace={ws}
                onClick={() => router.push(`/w/${ws.workspace_id}`)}
              />
            ))}
          </ul>
        )}
      </main>
    </div>
  );
}

function WorkspaceCard({ workspace, onClick }: { workspace: Workspace; onClick: () => void }) {
  return (
    <li>
      <button
        onClick={onClick}
        className="w-full rounded-xl border border-slate-200 bg-white px-5 py-4 shadow-sm text-left flex items-center gap-4 hover:border-indigo-300 hover:shadow-md transition-all group"
      >
        {/* Icon */}
        <div className="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg bg-indigo-50 group-hover:bg-indigo-100 transition-colors">
          <svg className="h-5 w-5 text-indigo-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M2.25 12.75V12A2.25 2.25 0 014.5 9.75h15A2.25 2.25 0 0121.75 12v.75m-8.69-6.44l-2.12-2.12a1.5 1.5 0 00-1.061-.44H4.5A2.25 2.25 0 002.25 6v12a2.25 2.25 0 002.25 2.25h15A2.25 2.25 0 0021.75 18V9a2.25 2.25 0 00-2.25-2.25h-5.379a1.5 1.5 0 01-1.06-.44z" />
          </svg>
        </div>

        {/* Info */}
        <div className="flex-1 min-w-0">
          <p className="font-semibold text-slate-800 truncate">{workspace.name}</p>
          <p className="mt-1 text-xs text-slate-400">作成: {formatDate(workspace.created_at)}</p>
        </div>

        {/* Arrow */}
        <svg className="h-4 w-4 text-slate-300 group-hover:text-indigo-400 transition-colors flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M8.25 4.5l7.5 7.5-7.5 7.5" />
        </svg>
      </button>
    </li>
  );
}
