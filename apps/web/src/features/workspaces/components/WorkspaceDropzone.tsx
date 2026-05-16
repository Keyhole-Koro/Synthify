import React from 'react';

interface WorkspaceDropzoneProps {
  isTreeMissing: boolean;
  hasChildItems: boolean;
  uploading: boolean;
  activeJobId: string | null;
  isDragging: boolean;
  jobStatusMessage?: string | null;
  jobStatusProgress?: number;
  jobStatusFailed?: boolean;
  onDragOver: (e: React.DragEvent) => void;
  onDragLeave: (e: React.DragEvent) => void;
  onDrop: (e: React.DragEvent) => void;
  onClick: () => void;
}

export function WorkspaceDropzone({
  isTreeMissing,
  hasChildItems,
  uploading,
  activeJobId,
  isDragging,
  jobStatusMessage,
  jobStatusProgress,
  jobStatusFailed,
  onDragOver,
  onDragLeave,
  onDrop,
  onClick,
}: WorkspaceDropzoneProps) {
  const showLargeDropzone = isTreeMissing || !hasChildItems;

  if (!showLargeDropzone) {
    return (
      <button
        type="button"
        disabled={uploading}
        onClick={onClick}
        className="flex w-full items-center justify-center gap-2 rounded-lg border border-dashed border-stone-200 py-2.5 text-xs font-medium text-stone-400 transition-colors hover:border-indigo-300 hover:text-indigo-500 disabled:cursor-wait disabled:opacity-60"
      >
        <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
        </svg>
        ファイルを追加
      </button>
    );
  }

  return (
    <div
      onDragOver={onDragOver}
      onDragLeave={onDragLeave}
      onDrop={onDrop}
      onClick={() => !uploading && !activeJobId && onClick()}
      className={[
        'group flex flex-col items-center justify-center gap-3 rounded-xl border-2 border-dashed px-6 py-10 text-center transition-all select-none',
        activeJobId ? 'cursor-default' : 'cursor-pointer',
        isDragging
          ? 'border-indigo-400 bg-indigo-50/60'
          : 'border-stone-200 hover:border-indigo-300 hover:bg-indigo-50/40',
        uploading ? 'cursor-wait opacity-60' : '',
      ].join(' ')}
    >
      <div className={[
        'flex h-11 w-11 items-center justify-center rounded-full transition-colors',
        isDragging ? 'bg-indigo-100' : 'bg-stone-100 group-hover:bg-indigo-100',
      ].join(' ')}>
        {uploading ? (
          <svg className="h-5 w-5 animate-spin text-indigo-500" fill="none" viewBox="0 0 24 24">
            <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
            <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z" />
          </svg>
        ) : activeJobId ? (
          <svg className="h-5 w-5 text-indigo-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        ) : (
          <svg className={['h-5 w-5 transition-colors', isDragging ? 'text-indigo-500' : 'text-stone-400 group-hover:text-indigo-500'].join(' ')} fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 16V4m0 0l-4 4m4-4l4 4M4 20h16" />
          </svg>
        )}
      </div>
      <div className="w-full">
        <p className="text-sm font-medium text-stone-700">
          {uploading ? 'アップロード中...' : activeJobId ? '解析中' : isDragging ? 'ここにドロップ' : 'ファイルをアップロード'}
        </p>
        {!uploading && !activeJobId && (
          <p className="mt-0.5 text-[11px] text-stone-400">クリックまたはドラッグ&ドロップ</p>
        )}
        {/* Inline progress for empty mode */}
        {activeJobId && (
          <div className="mt-3 w-full text-left">
            <div className="flex items-center justify-between gap-3">
              <span className="truncate text-[11px] text-stone-500">
                {jobStatusMessage ?? '解析中'}
              </span>
              {typeof jobStatusProgress === 'number' && (
                <span className="text-[10px] font-semibold text-stone-400">{jobStatusProgress}%</span>
              )}
            </div>
            <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-stone-100">
              <div
                className={[
                  'h-full rounded-full transition-all',
                  jobStatusFailed ? 'bg-red-400' : 'bg-indigo-500',
                ].join(' ')}
                style={{ width: `${Math.max(4, Math.min(100, jobStatusProgress ?? 8))}%` }}
              />
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
