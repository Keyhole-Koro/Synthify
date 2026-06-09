'use client';

import { useEffect, useState } from 'react';
import { resolveShareLink, type Workspace, WorkspaceRole } from '@/features/workspaces/api';
import { toAppError } from '@/lib/error_normalize';

interface ShareLinkViewerProps {
  token: string;
}

type ViewerState =
  | { status: 'loading' }
  | { status: 'resolved'; workspace: Workspace; role: WorkspaceRole }
  | { status: 'error'; message: string };

function roleLabel(role: WorkspaceRole): string {
  switch (role) {
    case WorkspaceRole.OWNER:
      return 'オーナー';
    case WorkspaceRole.EDITOR:
      return '編集可';
    case WorkspaceRole.VIEWER:
    default:
      return '閲覧のみ';
  }
}

export function ShareLinkViewer({ token }: ShareLinkViewerProps) {
  const [state, setState] = useState<ViewerState>({ status: 'loading' });

  useEffect(() => {
    let cancelled = false;
    resolveShareLink(token)
      .then((res) => {
        if (cancelled) return;
        setState({ status: 'resolved', workspace: res.workspace, role: res.role });
      })
      .catch((err) => {
        if (cancelled) return;
        const appErr = toAppError(err);
        const message =
          appErr.kind === 'not_found'
            ? 'この共有リンクは無効か、有効期限が切れています。'
            : appErr.message || '共有リンクを開けませんでした。';
        setState({ status: 'error', message });
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  if (state.status === 'loading') {
    return (
      <main className="flex min-h-screen items-center justify-center">
        <p className="text-sm text-slate-500">共有リンクを読み込んでいます…</p>
      </main>
    );
  }

  if (state.status === 'error') {
    return (
      <main className="flex min-h-screen items-center justify-center px-6">
        <div className="max-w-md text-center">
          <h1 className="text-lg font-semibold text-slate-900">リンクを開けません</h1>
          <p className="mt-2 text-sm text-slate-600">{state.message}</p>
        </div>
      </main>
    );
  }

  const { workspace, role } = state;
  return (
    <main className="mx-auto max-w-3xl px-6 py-12">
      <div className="mb-2 inline-flex items-center rounded-full bg-slate-100 px-3 py-1 text-xs font-medium text-slate-600">
        共有リンク · {roleLabel(role)}
      </div>
      <h1 className="text-2xl font-semibold text-slate-900">{workspace.name}</h1>
      <p className="mt-3 text-sm text-slate-600">
        このワークスペースは共有リンクで公開されています。閲覧専用です。
      </p>
    </main>
  );
}
