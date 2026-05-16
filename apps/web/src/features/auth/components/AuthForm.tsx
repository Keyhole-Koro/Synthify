import React from 'react';
import type { AuthMode } from '../AuthPaper';

interface AuthFormProps {
  mode: AuthMode;
  loading: boolean;
  onModeChange: (mode: AuthMode) => void;
  onEmailSubmit: () => void;
}

export function AuthForm({ mode, loading, onModeChange, onEmailSubmit }: AuthFormProps) {
  const isLogin = mode === 'login';
  function stop(e: React.PointerEvent) { e.stopPropagation(); }

  return (
    <div className="flex flex-col gap-4 pt-1">
      {/* Heading */}
      <div>
        <p className="text-[10px] font-semibold uppercase tracking-[0.18em] text-indigo-400/80">
          {isLogin ? 'Sign in' : 'Get started'}
        </p>
        <h3 className="mt-0.5 text-lg font-semibold tracking-tight text-stone-800">
          {isLogin ? 'おかえりなさい' : 'はじめよう'}
        </h3>
        <p className="mt-0.5 text-xs text-stone-400">
          {isLogin ? 'ドキュメントの知識ツリーを探索する' : '無料でワークスペースを作成する'}
        </p>
      </div>

      {/* Tab toggle */}
      <div className="flex rounded-lg bg-stone-100 p-1">
        {(['login', 'register'] as AuthMode[]).map((m) => (
          <button
            key={m}
            type="button"
            onPointerDown={stop} onPointerUp={stop}
            onClick={() => onModeChange(m)}
            className={[
              'flex-1 rounded-md py-1.5 text-xs font-semibold transition-all',
              mode === m
                ? 'bg-white text-stone-800 shadow-sm'
                : 'text-stone-400 hover:text-stone-600',
            ].join(' ')}
          >
            {m === 'login' ? 'ログイン' : '新規登録'}
          </button>
        ))}
      </div>

      {/* Fields */}
      <div className="flex flex-col gap-2.5">
        {!isLogin && (
          <div>
            <label className="mb-1 block text-[10px] font-semibold uppercase tracking-wider text-stone-400">お名前</label>
            <input
              type="text"
              placeholder="山田 太郎"
              onPointerDown={stop} onPointerUp={stop}
              className="w-full rounded-lg border border-stone-200 bg-white px-3 py-2 text-sm text-stone-800 placeholder-stone-300 outline-none focus:border-indigo-400 focus:ring-2 focus:ring-indigo-100"
            />
          </div>
        )}
        <div>
          <label className="mb-1 block text-[10px] font-semibold uppercase tracking-wider text-stone-400">メールアドレス</label>
          <input
            type="email"
            placeholder="you@example.com"
            onPointerDown={stop} onPointerUp={stop}
            className="w-full rounded-lg border border-stone-200 bg-white px-3 py-2 text-sm text-stone-800 placeholder-stone-300 outline-none focus:border-indigo-400 focus:ring-2 focus:ring-indigo-100"
          />
        </div>
        <div>
          <label className="mb-1 block text-[10px] font-semibold uppercase tracking-wider text-stone-400">パスワード</label>
          <input
            type="password"
            placeholder="••••••••"
            onPointerDown={stop} onPointerUp={stop}
            className="w-full rounded-lg border border-stone-200 bg-white px-3 py-2 text-sm text-stone-800 placeholder-stone-300 outline-none focus:border-indigo-400 focus:ring-2 focus:ring-indigo-100"
          />
        </div>
      </div>

      {/* Primary action */}
      <button
        type="button"
        onPointerDown={stop} onPointerUp={stop}
        onClick={onEmailSubmit}
        disabled={loading}
        className="w-full rounded-lg bg-indigo-500 py-2.5 text-sm font-semibold text-white transition-colors hover:bg-indigo-600 disabled:cursor-wait disabled:opacity-60"
      >
        {loading ? '処理中…' : isLogin ? 'ログイン' : 'アカウントを作成'}
      </button>
    </div>
  );
}
