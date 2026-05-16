import React from 'react';
import type { AuthUser } from '@/features/auth/session';

interface LoggedInProfileProps {
  user: AuthUser;
  onLogout: () => void;
}

export function LoggedInProfile({ user, onLogout }: LoggedInProfileProps) {
  function stop(e: React.PointerEvent) { e.stopPropagation(); }

  return (
    <div className="flex flex-col gap-4 pt-1">
      <div className="flex items-center gap-3">
        <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-indigo-100 text-sm font-semibold text-indigo-600">
          {(user.name || user.email).charAt(0).toUpperCase()}
        </div>
        <div className="min-w-0">
          <p className="truncate text-sm font-semibold text-stone-800">{user.name || user.email}</p>
          <p className="text-[11px] text-stone-400">ログイン中</p>
        </div>
      </div>
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
