import React from 'react';
import { type Workspace } from '@/features/workspaces/api';

interface WorkspaceItemListProps {
  workspaces: Workspace[];
  onOpenWorkspace: (workspaceId: string) => void;
}

export function WorkspaceItemList({ workspaces, onOpenWorkspace }: WorkspaceItemListProps) {
  if (workspaces.length === 0) return null;

  return (
    <div>
      <p className="mb-2 text-[10px] font-semibold uppercase tracking-[0.15em] text-stone-400">
        ワークスペース
      </p>
      <div className="flex flex-col gap-1">
        {workspaces.map((ws) => (
          <a
            key={ws.workspaceId}
            data-paper-id={ws.workspaceId}
            onPointerDown={(e) => { e.stopPropagation(); onOpenWorkspace(ws.workspaceId); }}
            onPointerUp={(e) => e.stopPropagation()}
            className="group flex items-center gap-3 rounded-lg border border-stone-100 bg-white px-3 py-2.5 text-sm font-medium text-stone-700 transition-all hover:border-indigo-200 hover:bg-indigo-50 hover:text-indigo-700 cursor-pointer"
          >
            <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-stone-100 transition-colors group-hover:bg-indigo-100">
              <svg className="h-3.5 w-3.5 text-stone-400 group-hover:text-indigo-500 transition-colors" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M3 7a2 2 0 012-2h4l2 2h8a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2V7z" />
              </svg>
            </div>
            <span className="truncate">{ws.name}</span>
            <svg className="ml-auto h-3.5 w-3.5 shrink-0 text-stone-300 opacity-0 transition-opacity group-hover:opacity-100 group-hover:text-indigo-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
            </svg>
          </a>
        ))}
      </div>
    </div>
  );
}
