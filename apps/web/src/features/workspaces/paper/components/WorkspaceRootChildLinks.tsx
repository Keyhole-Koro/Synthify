import React from 'react';

export interface WorkspaceRootChildLink {
  id: string;
  title: string;
}

interface WorkspaceRootChildLinksProps {
  children: WorkspaceRootChildLink[];
}

// WorkspaceRootChildLinks renders the workspace paper's child papers (the
// workspace's top-level knowledge nodes — projection rewrites their parentId to
// the workspaceId, so they sit directly under the workspace paper) as
// data-paper-id links *outside* the content iframe.
//
// The iframe (WorkspaceRootContent) is CSS-isolated, so any data-paper-id link
// inside it is cut off from paper-in-paper's click handler at the frame
// boundary. These links live in the host DOM, where the PaperContentReact
// wrapper's onClick walks up to [data-paper-id] and dispatches OPEN_NODE — the
// same mechanism WorkspaceItemList relies on.
export function WorkspaceRootChildLinks({ children }: WorkspaceRootChildLinksProps) {
  if (children.length === 0) {
    return null;
  }

  return (
    <div>
      <p className="mb-1.5 text-[10px] font-semibold uppercase tracking-[0.18em] text-stone-400">
        知識ノード
      </p>
      <div className="flex flex-wrap gap-1.5">
        {children.map((child) => (
          <a
            key={child.id}
            data-paper-id={child.id}
            onPointerDown={(e) => e.stopPropagation()}
            onPointerUp={(e) => e.stopPropagation()}
            className="group inline-flex items-center gap-1.5 rounded-full border border-stone-200 bg-white px-3 py-1 text-xs font-medium text-stone-600 transition-all hover:border-indigo-200 hover:bg-indigo-50 hover:text-indigo-700 cursor-pointer"
          >
            <span className="truncate max-w-[14rem]">{child.title || '無題のノード'}</span>
            <svg className="h-3 w-3 shrink-0 text-stone-300 transition-colors group-hover:text-indigo-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
            </svg>
          </a>
        ))}
      </div>
    </div>
  );
}
