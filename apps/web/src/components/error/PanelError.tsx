import React from 'react';
import { type AppError } from '@/lib/errors';

interface PanelErrorProps {
  error: AppError;
  onRetry?: () => void;
  onLogout?: () => void;
  className?: string;
}

export function PanelError({ error, onRetry, onLogout, className = '' }: PanelErrorProps) {
  function stop(e: React.PointerEvent) {
    e.stopPropagation();
  }

  return (
    <div className={`flex flex-col gap-3 pt-1 ${className}`}>
      <div className="rounded-lg border border-red-100 bg-red-50 px-3 py-2.5">
        <p className="text-xs font-semibold text-red-600">
          {error.kind === 'auth' ? '認証エラーが発生しました' : '読み込みに失敗しました'}
        </p>
        <p className="mt-0.5 text-[11px] leading-relaxed text-red-400">
          {error.message || 'しばらく時間をおいてから、再度お試しください。'}
        </p>
      </div>
      
      <div className="flex flex-col gap-2">
        {onRetry && (
          <button
            type="button"
            onPointerDown={stop}
            onPointerUp={stop}
            onClick={onRetry}
            className="w-full rounded-lg bg-white border border-stone-200 py-2 text-xs font-medium text-stone-700 transition-colors hover:bg-stone-50 active:bg-stone-100 shadow-sm"
          >
            再読み込み
          </button>
        )}
        
        {onLogout && (
          <button
            type="button"
            onPointerDown={stop}
            onPointerUp={stop}
            onClick={onLogout}
            className="w-full rounded-lg border border-transparent py-2 text-xs font-medium text-stone-500 transition-colors hover:text-stone-700"
          >
            ログアウト
          </button>
        )}
      </div>
    </div>
  );
}
