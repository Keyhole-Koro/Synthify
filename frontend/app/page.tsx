'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { listWorkspaces, createWorkspace, type Workspace } from '@/features/workspaces/api';

export default function HomePage() {
  const router = useRouter();
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState('');
  const [showCreate, setShowCreate] = useState(false);

  useEffect(() => {
    listWorkspaces()
      .then((ws) => {
        setWorkspaces(ws);
        // ワークスペースが1つだけなら自動リダイレクト
        if (ws.length === 1) {
          router.replace(`/w/${ws[0].workspace_id}`);
        }
      })
      .catch(console.error)
      .finally(() => setLoading(false));
  }, [router]);

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    if (!newName.trim()) return;
    setCreating(true);
    try {
      const ws = await createWorkspace(newName.trim());
      router.push(`/w/${ws.workspace_id}`);
    } catch (err) {
      console.error(err);
    } finally {
      setCreating(false);
    }
  }

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="h-6 w-6 animate-spin rounded-full border-2 border-indigo-600 border-t-transparent" />
      </div>
    );
  }

  return (
    <main className="mx-auto max-w-2xl px-6 py-16">
      <div className="mb-10 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Synthify</h1>
          <p className="mt-1 text-sm text-slate-500">ワークスペースを選択してください</p>
        </div>
        <button
          onClick={() => setShowCreate(true)}
          className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-700 transition-colors"
        >
          + 新規作成
        </button>
      </div>

      {showCreate && (
        <form onSubmit={handleCreate} className="mb-6 rounded-xl border border-slate-200 bg-white p-4 shadow-sm">
          <p className="mb-3 text-sm font-medium text-slate-700">新しいワークスペース</p>
          <div className="flex gap-2">
            <input
              autoFocus
              type="text"
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              placeholder="例: プロジェクト戦略"
              className="flex-1 rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none"
            />
            <button
              type="submit"
              disabled={creating || !newName.trim()}
              className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white disabled:opacity-50 hover:bg-indigo-700 transition-colors"
            >
              {creating ? '作成中…' : '作成'}
            </button>
            <button
              type="button"
              onClick={() => setShowCreate(false)}
              className="rounded-lg border border-slate-300 px-3 py-2 text-sm text-slate-600 hover:bg-slate-50 transition-colors"
            >
              キャンセル
            </button>
          </div>
        </form>
      )}

      {workspaces.length === 0 ? (
        <div className="rounded-xl border border-dashed border-slate-300 bg-white p-12 text-center">
          <p className="text-slate-500">ワークスペースがありません</p>
          <button
            onClick={() => setShowCreate(true)}
            className="mt-4 text-sm font-semibold text-indigo-600 hover:underline"
          >
            最初のワークスペースを作成する
          </button>
        </div>
      ) : (
        <ul className="space-y-3">
          {workspaces.map((ws) => (
            <li key={ws.workspace_id}>
              <a
                href={`/w/${ws.workspace_id}`}
                className="flex items-center justify-between rounded-xl border border-slate-200 bg-white px-5 py-4 shadow-sm hover:border-indigo-400 hover:shadow-md transition-all"
              >
                <div>
                  <p className="font-semibold text-slate-800">{ws.name}</p>
                  <p className="mt-0.5 text-xs text-slate-400">
                    {ws.plan === 'pro' ? '✦ Pro' : 'Free'} · {ws.workspace_id}
                  </p>
                </div>
                <span className="text-slate-400">→</span>
              </a>
            </li>
          ))}
        </ul>
      )}

      <div className="fixed bottom-8 right-8">
        <a
          href="/synthify"
          className="flex items-center gap-2 rounded-full bg-emerald-600 px-6 py-3 text-sm font-bold text-white shadow-xl transition-all hover:bg-emerald-700 hover:scale-105 active:scale-95"
        >
          <span>✨</span>
          Launch Paper Prototype
        </a>
      </div>
    </main>
  );
}
