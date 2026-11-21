import { buildPaperMap, PaperCanvas, type PaperCanvasHandle, type PaperCanvasLabels, type PaperMoveRequest } from '@keyhole-koro/paper-in-paper';
import { StrictMode, useEffect, useMemo, useRef, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { expansionFor, hashFromIds, idsFromHash, openIds, readTree, toPapers, type TreeFile } from './tree';

const JA_LABELS: PaperCanvasLabels = {
  closePaper: '閉じる',
  pinPaper: '固定する',
  unpinPaper: '固定をやめる',
  addChild: '子を追加',
  relatedLinks: '関連',
  collapseTo: (parentTitle) => `${parentTitle} に戻る`,
  paperTitleHint: 'Alt と矢印キーで、同じ階層の中で並べ替えます',
  contentBlocked: 'この内容は別のページを開こうとしたので、止めました。',
};

// A published tree cannot save moves: papers may be reordered in their room, not moved out of it.
const withinRoom = ({ sourceParentId, targetParentId }: PaperMoveRequest) => sourceParentId === targetParentId;

function openExternalLink(href: string) {
  if (/^(https?:|mailto:)/i.test(href)) window.open(href, '_blank', 'noopener,noreferrer');
}

function Viewer({ tree }: { tree: TreeFile }) {
  const paperMap = useMemo(() => buildPaperMap(toPapers(tree)), [tree]);
  const parentOf = useMemo(() => {
    const parents = new Map(tree.papers.map((p) => [p.id, p.parentId]));
    return (id: string) => parents.get(id);
  }, [tree]);
  const [defaultOpenState] = useState(() => ({ expansionMap: expansionFor(idsFromHash(location.hash), parentOf) }));
  const handleRef = useRef<PaperCanvasHandle>(null);

  useEffect(() => {
    const handle = handleRef.current;
    if (!handle) return;
    let written = location.hash;
    const unsubscribe = handle.subscribe(() => {
      const next = hashFromIds(openIds(handle.getState().expansionMap, tree.rootId));
      if (next === written) return;
      written = next;
      history.replaceState(history.state, '', next || location.pathname + location.search);
    });
    // A fragment typed into the address bar (SelfHostArtifacts passes it on) or a link to this page.
    const onHashChange = () => {
      if (location.hash === written) return;
      written = location.hash;
      handle.dispatch({ type: 'SET_OPEN_STATE', expansionMap: expansionFor(idsFromHash(location.hash), parentOf) });
    };
    window.addEventListener('hashchange', onHashChange);
    return () => {
      unsubscribe();
      window.removeEventListener('hashchange', onHashChange);
    };
  }, [tree, parentOf]);

  return (
    <PaperCanvas
      ref={handleRef}
      paperMap={paperMap}
      rootId={tree.rootId}
      defaultOpenState={defaultOpenState}
      colorScheme="system"
      labels={tree.lang.toLowerCase().startsWith('ja') ? JA_LABELS : undefined}
      canDrop={withinRoom}
      onOpenExternalLink={openExternalLink}
    />
  );
}

async function main() {
  const root = createRoot(document.getElementById('root')!);
  try {
    const res = await fetch('tree.json');
    if (!res.ok) throw new Error(`tree.json: HTTP ${res.status}`);
    const tree = readTree(await res.json());
    document.title = tree.title;
    document.documentElement.lang = tree.lang;
    root.render(
      <StrictMode>
        <Viewer tree={tree} />
      </StrictMode>,
    );
    document.documentElement.dataset.tree = 'ready';
  } catch (e) {
    root.render(<p className="status">Could not open this tree. {e instanceof Error ? e.message : String(e)}</p>);
    document.documentElement.dataset.tree = 'error';
  }
}

void main();
