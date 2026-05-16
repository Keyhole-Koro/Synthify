import React from 'react';

interface WorkspaceErrorProps {
  onLogout: () => void;
}

export function WorkspaceError({ onLogout }: WorkspaceErrorProps) {
  function stop(e: React.PointerEvent) { e.stopPropagation(); }

  return (
    <div className="flex flex-col gap-3 pt-1">
      <div className="rounded-lg border border-red-100 bg-red-50 px-3 py-2.5">
        <p className="text-xs font-semibold text-red-600">ワークスペースを読み込めませんでした</p>
        <p className="mt-0.5 text-[11px] text-red-400">しばらく時間をおいてから、再度お試しください。</p>
      </div>
      <button
        type="button"
        onPointerDown={stop} onPointerUp={stop}
        onClick={onLogout}
        className="w-full rounded-lg border border-stone-200 py-2 text-xs font-medium text-stone-500 transition-colors hover:bg-stone-50"
      >
        ログアウト
      </button>
    </div>
  );
}
