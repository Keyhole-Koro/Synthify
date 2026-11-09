import { useMemo } from 'react';
import { PaperHtmlContent } from '@keyhole-koro/paper-in-paper';

/**
 * Props for the {@link WorkspaceRootContent} sandboxed report iframe.
 */
interface WorkspaceRootContentProps {
  /** The workspace paper's node ID: links open relative to it and images load for it. */
  nodeId: string;
  /** The root node's rich HTML content (synthesised cover report). */
  content: string;
  /** Optional custom CSS stylesheet injected into the iframe. */
  overrideCss?: string | null;
  /** Child papers belonging to the workspace to append as fallback related links. */
  childItems?: { id: string; title: string }[];
}

/**
 * Report styles: a transparent page on the room's background, the app's Japanese font stack,
 * and the indigo paper-link pills. They replace paper-in-paper's default content styles.
 */
const REPORT_CSS = `
  :root { color-scheme: light; }
  body {
    font-family: ui-sans-serif, system-ui, -apple-system, "Hiragino Sans", "Noto Sans JP", sans-serif;
    color: #1c1917; padding: 16px 20px; line-height: 1.6; background: transparent;
  }
  img, table, pre { max-width: 100%; }
  a { color: #4f46e5; }
  a[data-paper-id] {
    display: inline-flex; align-items: center; gap: 0.3em; margin: 0 0.12em;
    padding: 0.1em 0.6em; border: 1px solid #c7d2fe; border-radius: 999px;
    background: #eef2ff; color: #4338ca; text-decoration: none; font-weight: 600;
    cursor: pointer;
  }
  a[data-paper-id]::after { content: '↗'; font-size: 0.82em; opacity: 0.7; }
  a[data-paper-id]:hover { background: #e0e7ff; }
  a[data-paper-id]:focus-visible { outline: 2px solid #6366f1; outline-offset: 2px; }
  .pip-related { margin-top: 16px; }
  .pip-related .eyebrow { text-transform: uppercase; letter-spacing: 0.18em; font-size: 10px; font-weight: 700; color: #a8a29e; margin: 0 0 6px; }
  .pip-related-links { display: flex; flex-wrap: wrap; gap: 6px; margin: 0; }
`;

/**
 * Renders the workspace cover report inside paper-in-paper's sandboxed content frame: scripts
 * in the (LLM-authored) HTML do not run, `data-paper-id` links open the child paper (or reveal
 * any other paper where it lives), images marked with `data-file-id` load through the canvas's
 * `loadImageUrl`, and children the report does not link to are appended as related links.
 */
export function WorkspaceRootContent({ nodeId, content, overrideCss, childItems = [] }: WorkspaceRootContentProps) {
  const relatedLinks = useMemo(
    () => childItems.map((child) => ({ id: child.id, title: child.title || '無題のノード' })),
    [childItems],
  );

  return (
    <PaperHtmlContent
      nodeId={nodeId}
      html={content}
      title="ワークスペースのレポート"
      relatedLinks={relatedLinks}
      baseCss={REPORT_CSS}
      overrideCss={overrideCss ?? undefined}
      className="w-full border-0 bg-transparent"
      data-testid="workspace-root-content"
    />
  );
}
