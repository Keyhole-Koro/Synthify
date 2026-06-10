'use client';

import { useCallback, useEffect, useState } from 'react';
import {
  createShareLink,
  listShareLinks,
  revokeShareLink,
  WorkspaceRole,
  type ShareLink,
} from '@/features/workspaces/api';
import { toAppError } from '@/lib/error_normalize';
import { InlineError } from '@/components/error/InlineError';

interface SharePanelProps {
  workspaceId: string;
  onClose: () => void;
}

// リンクに付与できる権限。owner は所有者に予約。
const LINK_ROLES: { value: WorkspaceRole; label: string }[] = [
  { value: WorkspaceRole.VIEWER, label: 'Read-only' },
  { value: WorkspaceRole.EDITOR, label: 'Read & write' },
];

function roleLabel(role: WorkspaceRole): string {
  return role === WorkspaceRole.EDITOR ? 'Read & write' : 'Read-only';
}

function shareLinkUrl(token: string): string {
  if (typeof window === 'undefined') return `/view/${token}`;
  return `${window.location.origin}/view/${token}`;
}

export function SharePanel({ workspaceId, onClose }: SharePanelProps) {
  const [links, setLinks] = useState<ShareLink[]>([]);
  const [role, setRole] = useState<WorkspaceRole>(WorkspaceRole.VIEWER);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [copiedToken, setCopiedToken] = useState('');

  const run = useCallback(async (fn: () => Promise<void>) => {
    setBusy(true);
    setError('');
    try {
      await fn();
    } catch (err) {
      setError(toAppError(err).message || '操作に失敗しました。');
    } finally {
      setBusy(false);
    }
  }, []);

  useEffect(() => {
    listShareLinks(workspaceId)
      .then(setLinks)
      .catch((err) => setError(toAppError(err).message || 'リンクの取得に失敗しました。'));
  }, [workspaceId]);

  const copy = useCallback(async (token: string) => {
    try {
      await navigator.clipboard?.writeText(shareLinkUrl(token));
      setCopiedToken(token);
      setTimeout(() => setCopiedToken((t) => (t === token ? '' : t)), 1500);
    } catch {
      // clipboard 不可時は無視 (URL は画面に出ているので手動コピー可能)。
    }
  }, []);

  // リンクを生成し、すぐクリップボードへコピーする (生成→コピーを一動作に)。
  const onGenerate = () =>
    run(async () => {
      const link = await createShareLink(workspaceId, role);
      setLinks((prev) => [link, ...prev]);
      await copy(link.token);
    });

  const onRevoke = (token: string) =>
    run(async () => {
      await revokeShareLink(workspaceId, token);
      setLinks((prev) => prev.map((l) => (l.token === token ? { ...l, revoked: true } : l)));
    });

  const activeLinks = links.filter((l) => !l.revoked);

  return (
    <div className="border-t border-stone-100 px-5 py-4 text-sm">
      <div className="mb-1 flex items-center justify-between">
        <h3 className="font-semibold text-stone-800">リンクで共有</h3>
        <button type="button" onClick={onClose} className="text-[11px] text-stone-400 hover:text-stone-600">
          閉じる
        </button>
      </div>
      <p className="mb-3 text-[11px] text-stone-400">
        リンクを知っている人がアクセスできます (ログイン不要)。
      </p>

      {error && <InlineError message={error} className="mb-2" />}

      {/* 入口は一つ: 権限を選んでリンクを生成 (生成と同時にコピー) */}
      <div className="flex items-center gap-2">
        <select
          value={role}
          onChange={(e) => setRole(Number(e.target.value) as WorkspaceRole)}
          disabled={busy}
          className="rounded-md border border-stone-200 px-2 py-1 text-[12px] disabled:opacity-60"
        >
          {LINK_ROLES.map((r) => (
            <option key={r.value} value={r.value}>
              {r.label}
            </option>
          ))}
        </select>
        <button
          type="button"
          onClick={onGenerate}
          disabled={busy}
          className="flex flex-1 items-center justify-center gap-1.5 rounded-md bg-indigo-500 px-3 py-1 text-[12px] font-medium text-white transition-colors hover:bg-indigo-600 disabled:opacity-50"
        >
          <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13.828 10.172a4 4 0 010 5.656l-3 3a4 4 0 01-5.656-5.656l1.5-1.5M10.172 13.828a4 4 0 010-5.656l3-3a4 4 0 015.656 5.656l-1.5 1.5" />
          </svg>
          リンク作成
        </button>
      </div>

      {activeLinks.length > 0 && (
        <ul className="mt-3 space-y-2">
          {activeLinks.map((l) => (
            <li
              key={l.token}
              className="group flex items-center gap-2 rounded-lg border border-stone-200/70 bg-stone-50/60 px-2.5 py-2 transition-colors hover:border-stone-300"
            >
              <span
                className={[
                  'shrink-0 rounded-full px-1.5 py-0.5 text-[10px] font-medium',
                  l.role === WorkspaceRole.EDITOR
                    ? 'bg-amber-100 text-amber-700'
                    : 'bg-stone-200/70 text-stone-600',
                ].join(' ')}
              >
                {roleLabel(l.role)}
              </span>
              <code className="min-w-0 flex-1 truncate font-mono text-[11px] text-stone-700">
                {shareLinkUrl(l.token)}
              </code>
              <button
                type="button"
                onClick={() => void copy(l.token)}
                title="コピー"
                className={[
                  'flex h-6 w-6 shrink-0 items-center justify-center rounded-md transition-colors',
                  copiedToken === l.token
                    ? 'text-emerald-500'
                    : 'text-stone-400 hover:bg-white hover:text-indigo-500',
                ].join(' ')}
              >
                {copiedToken === l.token ? (
                  <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                  </svg>
                ) : (
                  <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                  </svg>
                )}
              </button>
              <button
                type="button"
                onClick={() => onRevoke(l.token)}
                disabled={busy}
                title="リンクを失効"
                className="flex h-6 w-6 shrink-0 items-center justify-center rounded-md text-stone-400 transition-colors hover:bg-white hover:text-red-500 disabled:opacity-60"
              >
                <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                </svg>
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
