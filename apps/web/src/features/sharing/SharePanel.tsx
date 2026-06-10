'use client';

import { useCallback, useEffect, useState } from 'react';
import {
  inviteMember,
  updateMemberRole,
  removeMember,
  createShareLink,
  listShareLinks,
  revokeShareLink,
  WorkspaceRole,
  type WorkspaceMember,
  type ShareLink,
} from '@/features/workspaces/api';
import { toAppError } from '@/lib/error_normalize';
import { InlineError } from '@/components/error/InlineError';

interface SharePanelProps {
  workspaceId: string;
  initialMembers: WorkspaceMember[];
  onClose: () => void;
}

// 招待 / リンクで指定できるのは editor / viewer のみ (owner は所有者に予約)。
const ASSIGNABLE_ROLES: { value: WorkspaceRole; label: string }[] = [
  { value: WorkspaceRole.VIEWER, label: 'Read-only' },
  { value: WorkspaceRole.EDITOR, label: 'Read & write' },
];

function roleLabel(role: WorkspaceRole): string {
  switch (role) {
    case WorkspaceRole.OWNER:
      return 'オーナー';
    case WorkspaceRole.EDITOR:
      return 'Read & write';
    default:
      return 'Read-only';
  }
}

function shareLinkUrl(token: string): string {
  if (typeof window === 'undefined') return `/view/${token}`;
  return `${window.location.origin}/view/${token}`;
}

export function SharePanel({ workspaceId, initialMembers, onClose }: SharePanelProps) {
  const [members, setMembers] = useState<WorkspaceMember[]>(initialMembers);
  const [links, setLinks] = useState<ShareLink[]>([]);
  const [email, setEmail] = useState('');
  const [inviteRole, setInviteRole] = useState<WorkspaceRole>(WorkspaceRole.VIEWER);
  const [linkRole, setLinkRole] = useState<WorkspaceRole>(WorkspaceRole.VIEWER);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [showLinks, setShowLinks] = useState(false);

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

  const onInvite = (e: React.FormEvent) => {
    e.preventDefault();
    const trimmed = email.trim();
    if (!trimmed) return;
    void run(async () => {
      const member = await inviteMember(workspaceId, trimmed, inviteRole);
      setMembers((prev) => {
        const rest = prev.filter((m) => m.userId !== member.userId);
        return [...rest, member];
      });
      setEmail('');
    });
  };

  const onChangeRole = (userId: string, role: WorkspaceRole) =>
    run(async () => {
      const member = await updateMemberRole(workspaceId, userId, role);
      setMembers((prev) => prev.map((m) => (m.userId === userId ? member : m)));
    });

  const onRemove = (userId: string) =>
    run(async () => {
      await removeMember(workspaceId, userId);
      setMembers((prev) => prev.filter((m) => m.userId !== userId));
    });

  const onCreateLink = () =>
    run(async () => {
      const link = await createShareLink(workspaceId, linkRole);
      setLinks((prev) => [link, ...prev]);
    });

  const onRevoke = (token: string) =>
    run(async () => {
      await revokeShareLink(workspaceId, token);
      setLinks((prev) => prev.map((l) => (l.token === token ? { ...l, revoked: true } : l)));
    });

  const activeLinks = links.filter((l) => !l.revoked);

  return (
    <div className="border-t border-stone-100 px-5 py-4 text-sm">
      <div className="mb-3 flex items-center justify-between">
        <h3 className="font-semibold text-stone-800">共有</h3>
        <button type="button" onClick={onClose} className="text-[11px] text-stone-400 hover:text-stone-600">
          閉じる
        </button>
      </div>

      {error && <InlineError message={error} className="mb-2" />}

      {/* メンバー招待 */}
      <form onSubmit={onInvite} className="flex items-center gap-2">
        <input
          type="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          placeholder="招待するメールアドレス"
          disabled={busy}
          className="min-w-0 flex-1 rounded-md border border-stone-200 px-2 py-1 outline-none focus:border-indigo-400 disabled:opacity-60"
        />
        <select
          value={inviteRole}
          onChange={(e) => setInviteRole(Number(e.target.value) as WorkspaceRole)}
          disabled={busy}
          className="rounded-md border border-stone-200 px-2 py-1 text-[12px] disabled:opacity-60"
        >
          {ASSIGNABLE_ROLES.map((r) => (
            <option key={r.value} value={r.value}>
              {r.label}
            </option>
          ))}
        </select>
        <button
          type="submit"
          disabled={busy || !email.trim()}
          className="rounded-md bg-indigo-500 px-3 py-1 text-[12px] font-medium text-white disabled:opacity-50"
        >
          招待
        </button>
      </form>

      <ul className="mt-3 space-y-1">
        {members.map((m) => (
          <li key={m.userId} className="flex items-center justify-between gap-2">
            <span className="min-w-0 truncate text-stone-700">{m.email || m.userId}</span>
            <div className="flex shrink-0 items-center gap-2">
              {m.role === WorkspaceRole.OWNER ? (
                <span className="text-[11px] text-stone-400">{roleLabel(m.role)}</span>
              ) : (
                <>
                  <select
                    value={m.role}
                    onChange={(e) => onChangeRole(m.userId, Number(e.target.value) as WorkspaceRole)}
                    disabled={busy}
                    className="rounded-md border border-stone-200 px-1.5 py-0.5 text-[11px] disabled:opacity-60"
                  >
                    {ASSIGNABLE_ROLES.map((r) => (
                      <option key={r.value} value={r.value}>
                        {r.label}
                      </option>
                    ))}
                  </select>
                  <button
                    type="button"
                    onClick={() => onRemove(m.userId)}
                    disabled={busy}
                    className="text-[11px] text-red-500 hover:text-red-600 disabled:opacity-60"
                  >
                    削除
                  </button>
                </>
              )}
            </div>
          </li>
        ))}
      </ul>

      {/* 公開リンク (折りたたみ。メンバー招待を主体にし、リンクは必要時に展開) */}
      <div className="mt-4 border-t border-stone-100 pt-3">
        <button
          type="button"
          onClick={() => setShowLinks((v) => !v)}
          className="flex w-full items-center justify-between text-[12px] text-stone-500 hover:text-stone-700"
        >
          <span className="flex items-center gap-1.5">
            <svg
              className={['h-3 w-3 transition-transform', showLinks ? 'rotate-90' : ''].join(' ')}
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
            </svg>
            リンクで共有
          </span>
          {activeLinks.length > 0 && (
            <span className="rounded-full bg-stone-100 px-1.5 text-[10px] text-stone-500">
              {activeLinks.length}
            </span>
          )}
        </button>

        {showLinks && (
          <div className="mt-2">
            <p className="mb-2 text-[11px] text-stone-400">
              リンクを知っている人は誰でも閲覧できます (ログイン不要)。
            </p>
            <div className="mb-2 flex items-center justify-end gap-2">
              <select
                value={linkRole}
                onChange={(e) => setLinkRole(Number(e.target.value) as WorkspaceRole)}
                disabled={busy}
                className="rounded-md border border-stone-200 px-1.5 py-0.5 text-[11px] disabled:opacity-60"
              >
                {ASSIGNABLE_ROLES.map((r) => (
                  <option key={r.value} value={r.value}>
                    {r.label}
                  </option>
                ))}
              </select>
              <button
                type="button"
                onClick={onCreateLink}
                disabled={busy}
                className="rounded-md border border-indigo-200 px-2 py-0.5 text-[11px] font-medium text-indigo-600 disabled:opacity-60"
              >
                リンク発行
              </button>
            </div>
            {activeLinks.length === 0 ? (
              <p className="text-[11px] text-stone-400">有効なリンクはありません。</p>
            ) : (
              <ul className="space-y-1">
                {activeLinks.map((l) => (
                  <li key={l.token} className="flex items-center justify-between gap-2">
                    <span className="min-w-0 flex-1 truncate text-[11px] text-stone-500">
                      {shareLinkUrl(l.token)} · {roleLabel(l.role)}
                    </span>
                    <div className="flex shrink-0 items-center gap-2">
                      <button
                        type="button"
                        onClick={() => void navigator.clipboard?.writeText(shareLinkUrl(l.token))}
                        className="text-[11px] text-indigo-500 hover:text-indigo-600"
                      >
                        コピー
                      </button>
                      <button
                        type="button"
                        onClick={() => onRevoke(l.token)}
                        disabled={busy}
                        className="text-[11px] text-red-500 hover:text-red-600 disabled:opacity-60"
                      >
                        失効
                      </button>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
