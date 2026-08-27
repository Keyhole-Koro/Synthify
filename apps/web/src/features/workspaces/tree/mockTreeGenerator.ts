import { create } from '@bufbuild/protobuf';
import { ItemSchema } from '@/gen/proto/synthify/app/v1/tree_types_pb';
import type { ApiItem } from '@/features/tree/api';

// mockTreeGenerator builds workspace trees of an arbitrary, precisely
// controlled size without touching the backend. It is the single source of
// mock trees for three consumers:
//
//   - __synthifyDebug.createMockWorkspace — previewing UI states from the console
//   - treeLoad.bench.ts — the L0 data-layer benchmark (no browser)
//   - e2e/perf.spec.ts — the L1 in-browser load test
//
// Sharing one generator is what makes the L0 numbers comparable to the L1
// numbers: both are fed byte-identical trees.
//
// The knobs are deliberately orthogonal, because they stress different parts
// of the client:
//
//   totalItems   → cache size, projection cost, GetTree payload item count
//   depth        → recursion / subtree-load chains, paper nesting
//   branching    → sibling count per paper (paper-in-paper room allocation)
//   contentBytes → payload bytes and retained memory (content is HTML held
//                  for every item, not just the open ones)
//
// Generation is deterministic for a given (workspaceId, spec): same ids, same
// titles, same content bytes. Benchmarks that are not reproducible are not
// benchmarks.

export interface MockTreeSpec {
  // totalItems caps the node count, root included. When omitted, the full
  // balanced tree implied by depth/branching is generated.
  totalItems?: number;
  // depth is the number of levels *below* the root. depth=0 is a lone root.
  depth?: number;
  // branching is the number of children per non-leaf node.
  branching?: number;
  // contentBytes pads each node's HTML content to roughly this many bytes.
  // 0 (the default) keeps the short natural content.
  contentBytes?: number;
  seed?: number;
  titlePrefix?: string;
  // rootContent / rootOverrideCss override the root node's cover-report HTML
  // and isolated CSS. When omitted, a rich sample report is used so the iframe
  // rendering, CSS isolation, and child links are all exercised.
  rootContent?: string;
  rootOverrideCss?: string;
}

// ResolvedMockTreeSpec is the spec after defaults are applied, plus the shape
// that was actually produced. Reported alongside measurements so a number is
// never ambiguous about the tree it came from.
export interface ResolvedMockTreeSpec {
  totalItems: number;
  depth: number;
  branching: number;
  contentBytes: number;
  seed: number;
  levelCounts: number[];
  contentTotalBytes: number;
}

const DEFAULT_DEPTH = 2;
const DEFAULT_BRANCHING = 3;
const DEFAULT_SEED = 1;

// SAMPLE_ROOT_OVERRIDE_CSS is injected into the root content iframe so the CSS
// isolation path is exercised: these selectors would clash with the host page
// if they leaked, but stay contained inside the sandboxed iframe.
export const SAMPLE_ROOT_OVERRIDE_CSS = `
.hero-block { background: linear-gradient(135deg, #eef2ff, #fdf2f8); border-radius: 16px; padding: 18px 20px; }
h1 { color: #4338ca; }
.metric { display: inline-block; margin-right: 16px; font-weight: 700; color: #be185d; }
`.trim();

// mulberry32 is a small deterministic PRNG. Content filler is varied rather
// than a repeated constant so that payload measurements are not flattered by
// compression of a single repeating token.
function mulberry32(seed: number): () => number {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

const FILLER = [
  '知識ノード', '抽象', '具体', '要約', '出典', '統合', '観測', '仮説', '検証', '設計',
  'chunking', 'embedding', 'retrieval', 'synthesis', 'critique', 'evidence', 'ontology',
  'workspace', 'document', 'projection', 'expansion', 'canvas', 'iframe', 'payload',
];

// buildSampleRootContent renders a rich cover report that embeds child node ids
// as data-paper-id links directly in the content. WorkspaceRootContent forwards
// in-iframe clicks on these links to the host, which opens the child paper — so
// the embedded links are live navigation, exercising that path from the console.
export function buildSampleRootContent(
  childRefs: { id: string; title: string }[],
  totalItems = childRefs.length + 1,
): string {
  const links = childRefs
    .map((c) => `<a data-paper-id="${c.id}">${c.title}</a>`)
    .join(' ');
  return `
<div class="hero-block">
  <h1>統合知識レポート（モック）</h1>
  <p class="lede">__synthifyDebug が生成したフロントエンド専用のダミー root content です。リッチ HTML + override_css が iframe で CSS 隔離描画されます。</p>
  <p><span class="metric">出典 3</span><span class="metric">ノード ${totalItems}</span></p>
</div>
<h2>概要</h2>
<p>このワークスペースの知識ツリーのトップ概要。下のリンクから配下のノードを開けます。</p>
<table><thead><tr><th>項目</th><th>値</th></tr></thead><tbody>
<tr><td>モデル</td><td>node 直属</td></tr><tr><td>root</td><td>単一</td></tr></tbody></table>
<h2>子ノード（クリックで開く）</h2>
<p>${links || '（子ノードなし）'}</p>
`.trim();
}

// buildContent pads a node's HTML to approximately contentBytes. The padding is
// wrapped in real markup (paragraphs, a list) rather than one giant text node,
// so the browser-side cost of parsing it into the content iframe resembles
// real LLM-generated node content.
function buildContent(title: string, contentBytes: number, rand: () => number): string {
  const head = `<h2>${title}</h2><p>${title} の本文。モックツリー生成器が作ったダミーです。</p>`;
  if (contentBytes <= 0) return head;

  const parts: string[] = [head];
  let size = head.length;
  let paragraph: string[] = [];
  while (size < contentBytes) {
    paragraph.push(FILLER[Math.floor(rand() * FILLER.length)]);
    if (paragraph.length >= 24) {
      const block = `<p>${paragraph.join(' ')}</p>`;
      parts.push(block);
      size += block.length;
      paragraph = [];
    }
  }
  if (paragraph.length > 0) parts.push(`<p>${paragraph.join(' ')}</p>`);
  return parts.join('');
}

interface PlannedNode {
  id: string;
  parentId: string;
  level: number;
  childIds: string[];
}

// planTree lays out the node graph breadth-first: every node gets `branching`
// children until either `depth` or `totalItems` is reached. Breadth-first (not
// depth-first) truncation keeps a capped tree balanced, so `totalItems` alone
// is a meaningful knob — the shape does not lurch as the cap moves.
function planTree(idPrefix: string, totalItems: number, depth: number, branching: number): PlannedNode[] {
  const root: PlannedNode = { id: `${idPrefix}_0`, parentId: '', level: 0, childIds: [] };
  const nodes: PlannedNode[] = [root];
  if (totalItems <= 1) return nodes;

  let cursor = 0;
  while (cursor < nodes.length && nodes.length < totalItems) {
    const parent = nodes[cursor++];
    if (parent.level >= depth) continue;
    for (let i = 0; i < branching && nodes.length < totalItems; i++) {
      const child: PlannedNode = {
        id: `${idPrefix}_${nodes.length}`,
        parentId: parent.id,
        level: parent.level + 1,
        childIds: [],
      };
      parent.childIds.push(child.id);
      nodes.push(child);
    }
  }
  return nodes;
}

function fullTreeSize(depth: number, branching: number): number {
  let size = 1;
  let levelSize = 1;
  for (let level = 0; level < depth; level++) {
    levelSize *= branching;
    size += levelSize;
  }
  return size;
}

export function resolveMockTreeSpec(spec: MockTreeSpec): Omit<ResolvedMockTreeSpec, 'levelCounts' | 'contentTotalBytes'> {
  const depth = Math.max(0, Math.floor(spec.depth ?? DEFAULT_DEPTH));
  const branching = Math.max(1, Math.floor(spec.branching ?? DEFAULT_BRANCHING));
  const requested = spec.totalItems;
  const totalItems = requested != null
    ? Math.max(1, Math.floor(requested))
    : fullTreeSize(depth, branching);
  return {
    totalItems,
    depth,
    branching,
    contentBytes: Math.max(0, Math.floor(spec.contentBytes ?? 0)),
    seed: spec.seed ?? DEFAULT_SEED,
  };
}

export interface MockTreeResult {
  items: ApiItem[];
  resolved: ResolvedMockTreeSpec;
}

// buildMockTree assembles a complete ApiItem[] for a single-root workspace
// tree, mirroring the worker's node-direct model: one root node (parent_id =
// '') carrying the cover-report content, with knowledge nodes hanging beneath
// it. The shape mirrors what GetTree returns, so it flows through
// replaceWorkspaceTree unchanged.
export function buildMockTree(workspaceId: string, spec: MockTreeSpec = {}): MockTreeResult {
  const { totalItems, depth, branching, contentBytes, seed } = resolveMockTreeSpec(spec);
  const rand = mulberry32(seed);
  const idPrefix = `mock_${workspaceId}`;
  const prefix = spec.titlePrefix?.trim() || '知識ノード';

  const planned = planTree(idPrefix, totalItems, depth, branching);
  const titleById = new Map<string, string>();
  planned.forEach((node, index) => {
    titleById.set(node.id, index === 0 ? 'モック ワークスペース' : `${prefix} ${index}`);
  });

  const levelCounts: number[] = [];
  let contentTotalBytes = 0;

  const items = planned.map((node, index) => {
    const title = titleById.get(node.id)!;
    levelCounts[node.level] = (levelCounts[node.level] ?? 0) + 1;

    const content = index === 0
      ? (spec.rootContent ?? buildSampleRootContent(
          node.childIds.map((id) => ({ id, title: titleById.get(id)! })),
          totalItems,
        ))
      : buildContent(title, contentBytes, rand);
    contentTotalBytes += content.length;

    return create(ItemSchema, {
      id: node.id,
      parentId: node.parentId,
      title,
      description: index === 0 ? 'Mock single root node' : `Mock node ${index} (level ${node.level})`,
      content,
      overrideCss: index === 0 ? (spec.rootOverrideCss ?? SAMPLE_ROOT_OVERRIDE_CSS) : '',
      level: node.level,
      childIds: node.childIds,
    });
  });

  return {
    items,
    resolved: { totalItems: items.length, depth, branching, contentBytes, seed, levelCounts, contentTotalBytes },
  };
}
