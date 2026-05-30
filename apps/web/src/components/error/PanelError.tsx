import React from 'react';
import { type AppError } from '@/lib/errors';

interface PanelErrorProps {
  error: AppError;
  className?: string;
}

export function PanelError({ error, className = '' }: PanelErrorProps) {
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
    </div>
  );
}
