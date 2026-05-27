import React from 'react';

interface FatalErrorViewProps {
  error: Error & { digest?: string };
  reset: () => void;
}

export function FatalErrorView({ error, reset }: FatalErrorViewProps) {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-slate-50 px-6 text-center">
      <div className="mb-6 flex h-16 w-16 items-center justify-center rounded-full bg-red-100 text-red-600">
        <svg className="h-8 w-8" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
          />
        </svg>
      </div>
      
      <h1 className="mb-2 text-2xl font-bold text-slate-900">
        申し訳ありません。エラーが発生しました。
      </h1>
      
      <p className="mb-8 max-w-md text-slate-500">
        アプリの読み込み中に予期しない問題が発生しました。
        一時的な問題の可能性がありますので、再読み込みをお試しください。
      </p>

      {error.digest && (
        <p className="mb-8 text-[10px] font-mono text-slate-400">
          Error ID: {error.digest}
        </p>
      )}

      <div className="flex flex-col gap-3 sm:flex-row">
        <button
          onClick={reset}
          className="inline-flex items-center justify-center rounded-lg bg-slate-900 px-6 py-2.5 text-sm font-semibold text-white transition-colors hover:bg-slate-800 focus:outline-none focus:ring-2 focus:ring-slate-900 focus:ring-offset-2"
        >
          再読み込み
        </button>
        <a
          href="/"
          className="inline-flex items-center justify-center rounded-lg border border-slate-200 bg-white px-6 py-2.5 text-sm font-semibold text-slate-700 transition-colors hover:bg-slate-50 focus:outline-none focus:ring-2 focus:ring-slate-200 focus:ring-offset-2"
        >
          ホームに戻る
        </a>
      </div>
    </div>
  );
}
