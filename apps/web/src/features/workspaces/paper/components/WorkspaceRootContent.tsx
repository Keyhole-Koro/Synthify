import React, { useMemo, useRef, useState } from 'react';

interface WorkspaceRootContentProps {
  // content is the root node's rich HTML (the workspace's cover report).
  content: string;
  // overrideCss is the node's per-item CSS, isolated inside the iframe so it
  // never collides with the host page.
  overrideCss?: string | null;
}

// WorkspaceRootContent renders the workspace's single root node content inside
// a sandboxed iframe. The iframe is the reason this is a separate component:
// the root node carries its own override_css and arbitrary rich HTML, which we
// must render CSS-isolated from (and unable to script) the host page.
export function WorkspaceRootContent({ content, overrideCss }: WorkspaceRootContentProps) {
  const frameRef = useRef<HTMLIFrameElement>(null);
  const [height, setHeight] = useState(160);

  const srcDoc = useMemo(() => {
    // Minimal reset + the node's override CSS, then the content. A tiny script
    // posts the rendered height back so the iframe can size to its content.
    return `<!doctype html><html><head><meta charset="utf-8" />
<style>
  :root { color-scheme: light; }
  html, body { margin: 0; padding: 0; }
  body { font-family: ui-sans-serif, system-ui, -apple-system, "Noto Sans JP", sans-serif; color: #1c1917; padding: 16px 20px; line-height: 1.6; }
  img, table, pre { max-width: 100%; }
  a { color: #4f46e5; }
${overrideCss ?? ''}
</style></head>
<body>${content}
<script>
  function report() {
    var h = document.body.scrollHeight;
    parent.postMessage({ __synthifyRootContentHeight: h }, '*');
  }
  window.addEventListener('load', report);
  new ResizeObserver(report).observe(document.body);
</script>
</body></html>`;
  }, [content, overrideCss]);

  React.useEffect(() => {
    function onMessage(e: MessageEvent) {
      const h = (e.data as { __synthifyRootContentHeight?: number })?.__synthifyRootContentHeight;
      if (typeof h === 'number' && h > 0 && Math.abs(h - height) > 4) {
        setHeight(Math.min(h, 600));
      }
    }
    window.addEventListener('message', onMessage);
    return () => window.removeEventListener('message', onMessage);
  }, [height]);

  return (
    <iframe
      ref={frameRef}
      title="workspace-root-content"
      // sandbox allows the resize script but blocks top navigation, forms,
      // popups, and same-origin access — untrusted LLM HTML stays contained.
      sandbox="allow-scripts"
      srcDoc={srcDoc}
      className="w-full rounded-lg border border-stone-100 bg-white"
      style={{ height }}
    />
  );
}
