import React, { useMemo, useRef, useState } from 'react';
import { usePaperDispatch } from '@keyhole-koro/paper-in-paper';

interface WorkspaceRootContentProps {
  // nodeId is the workspace paper's id. data-paper-id clicks inside the iframe
  // open child papers via dispatch({ OPEN_NODE, parentId: nodeId, childId }).
  nodeId: string;
  // content is the root node's rich HTML (the workspace's cover report).
  content: string;
  // overrideCss is the node's per-item CSS, isolated inside the iframe so it
  // never collides with the host page.
  overrideCss?: string | null;
  // childItems are the workspace paper's child papers (top-level knowledge
  // nodes). Any child not already referenced by a data-paper-id link in the
  // content is appended as a "related" link so navigation is never lost.
  childItems?: { id: string; title: string }[];
}

// appendRelatedLinks mirrors paper-in-paper's behavior: children not mentioned
// in the content HTML get appended as data-paper-id links so every child stays
// reachable from the cover report.
function appendRelatedLinks(content: string, childItems: { id: string; title: string }[]): string {
  const unmentioned = childItems.filter((c) => !content.includes(`data-paper-id="${c.id}"`));
  if (unmentioned.length === 0) return content;
  const links = unmentioned
    .map((c) => `<a data-paper-id="${c.id}">${c.title || '無題のノード'}</a>`)
    .join('');
  return `${content}<hr/><div class="related"><p class="related-label">関連ノード</p><p class="related-links">${links}</p></div>`;
}

// WorkspaceRootContent renders the workspace's single root node content inside
// a sandboxed iframe. The iframe is the reason this is a separate component:
// the root node carries its own override_css and arbitrary rich HTML, which we
// must render CSS-isolated from (and unable to script) the host page.
//
// A small bootstrap script inside the iframe forwards data-paper-id clicks to
// the host via postMessage (the frame boundary otherwise cuts those clicks off
// from paper-in-paper). The host turns each into an OPEN_NODE dispatch, so
// child links embedded directly in the LLM-authored content are clickable.
export function WorkspaceRootContent({ nodeId, content, overrideCss, childItems = [] }: WorkspaceRootContentProps) {
  const dispatch = usePaperDispatch();
  const frameRef = useRef<HTMLIFrameElement>(null);
  const [height, setHeight] = useState(160);

  const srcDoc = useMemo(() => {
    const finalContent = appendRelatedLinks(content, childItems);
    // Minimal reset + the node's override CSS, then the content. The bootstrap
    // script posts the rendered height back (so the iframe can size to its
    // content) and forwards data-paper-id clicks to the host.
    return `<!doctype html><html><head><meta charset="utf-8" />
<style>
  :root { color-scheme: light; }
  html, body { margin: 0; padding: 0; }
  body { font-family: ui-sans-serif, system-ui, -apple-system, "Noto Sans JP", sans-serif; color: #1c1917; padding: 16px 20px; line-height: 1.6; background: transparent; }
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
  .related { margin-top: 16px; }
  .related-label { text-transform: uppercase; letter-spacing: 0.18em; font-size: 10px; font-weight: 700; color: #a8a29e; margin-bottom: 6px; }
  .related-links { display: flex; flex-wrap: wrap; gap: 6px; }
${overrideCss ?? ''}
</style></head>
<body>${finalContent}
<script>
  function report() {
    parent.postMessage({ __synthifyRootContentHeight: document.body.scrollHeight }, '*');
  }
  window.addEventListener('load', report);
  new ResizeObserver(report).observe(document.body);
  document.addEventListener('click', function (e) {
    var el = e.target.closest('[data-paper-id]');
    if (!el) return;
    e.preventDefault();
    parent.postMessage({ __synthifyOpenPaperId: el.getAttribute('data-paper-id') }, '*');
  });
</script>
</body></html>`;
  }, [content, overrideCss, childItems]);

  React.useEffect(() => {
    function onMessage(e: MessageEvent) {
      const data = e.data as { __synthifyRootContentHeight?: number; __synthifyOpenPaperId?: string };
      const h = data?.__synthifyRootContentHeight;
      if (typeof h === 'number' && h > 0 && Math.abs(h - height) > 4) {
        // Size the iframe to its full content height so it never scrolls
        // internally; the surrounding paper room handles overflow instead.
        setHeight(h);
      }
      const childId = data?.__synthifyOpenPaperId;
      if (typeof childId === 'string' && childId) {
        dispatch({ type: 'OPEN_NODE', parentId: nodeId, childId });
      }
    }
    window.addEventListener('message', onMessage);
    return () => window.removeEventListener('message', onMessage);
  }, [height, dispatch, nodeId]);

  return (
    <iframe
      ref={frameRef}
      title="workspace-root-content"
      // sandbox allows the resize/click script but blocks top navigation, forms,
      // popups, and same-origin access — untrusted LLM HTML stays contained.
      sandbox="allow-scripts"
      srcDoc={srcDoc}
      // Transparent body + transparent iframe: the host paper's room background
      // shows through, so the content blends into the room with no seam. Content
      // that wants its own background (e.g. a hero block) still paints it.
      className="w-full border-0 bg-transparent"
      style={{ height }}
    />
  );
}
