import React from 'react';

export interface WorkspaceDocumentItem {
  id: string;
  title: string;
}

interface WorkspaceDocumentListProps {
  documents: WorkspaceDocumentItem[];
}

// WorkspaceDocumentList renders the workspace's document_root items as cards
// inside the workspace paper's content. Each card carries data-paper-id so
// paper-in-paper opens the corresponding child paper on click (same mechanism
// as WorkspaceItemList in the root content). Opening behavior, projection and
// layout are unchanged — these cards are just an in-content entry point to the
// document roots that already exist as child papers.
export function WorkspaceDocumentList({ documents }: WorkspaceDocumentListProps) {
  if (documents.length === 0) {
    return null;
  }

  return (
    <div>
      <p className="mb-1.5 text-[10px] font-semibold uppercase tracking-[0.18em] text-stone-400">
        ドキュメント
      </p>
      <div className="flex flex-col gap-1">
        {documents.map((doc) => (
          <a
            key={doc.id}
            data-paper-id={doc.id}
            onPointerDown={(e) => e.stopPropagation()}
            onPointerUp={(e) => e.stopPropagation()}
            className="group flex items-center gap-3 rounded-lg border border-stone-100 bg-white px-3 py-2.5 text-sm font-medium text-stone-700 transition-all hover:border-indigo-200 hover:bg-indigo-50 hover:text-indigo-700 cursor-pointer"
          >
            <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-stone-100 transition-colors group-hover:bg-indigo-100">
              <svg className="h-3.5 w-3.5 text-stone-400 group-hover:text-indigo-500 transition-colors" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              </svg>
            </div>
            <span className="truncate">{doc.title || '無題のドキュメント'}</span>
            <svg className="ml-auto h-3.5 w-3.5 shrink-0 text-stone-300 opacity-0 transition-opacity group-hover:opacity-100 group-hover:text-indigo-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
            </svg>
          </a>
        ))}
      </div>
    </div>
  );
}
