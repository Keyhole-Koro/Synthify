import { bench, describe } from 'vitest';
import { gzipSync } from 'node:zlib';
import { create, toJson, fromJson, toBinary, fromBinary } from '@bufbuild/protobuf';
import type { Paper } from '@keyhole-koro/paper-in-paper';
import { GetTreeResponseSchema } from '@/gen/proto/synthify/app/v1/tree_pb';
import { ItemSchema, TreeSchema } from '@/gen/proto/synthify/app/v1/tree_types_pb';
import { projectWorkspacePapers } from '@/features/workspaces/useWorkspaceProjection';
import { buildMockTree } from './mockTreeGenerator';
import { createWorkspaceTreeCache } from './workspaceTreeCache';

// L0 of the client load test: the data-layer cost of holding many items,
// measured without a browser. See docs/improvements/client-item-load-testing.md.
//
// Run with `bun run bench`. These are the three things that scale with item
// count on every workspace open and every paper expansion:
//
//   1. decode   — connect-web is on the JSON codec (lib/connect.ts sets no
//                 useBinaryFormat), so GetTree costs a JSON.parse plus a
//                 protobuf fromJson over every item, content included.
//   2. cache    — replaceWorkspaceTree rebuilds the whole per-workspace Map
//                 and allocates a SubtreeItem message per item.
//   3. project  — projectWorkspacePapers rebuilds *every* Paper for the
//                 workspace. It runs again on each expansion and each
//                 treeChanged refresh, so this is a per-interaction cost, not
//                 a one-off load cost.
//
// SCALES pairs an item count with a content size. Real nodes carry
// LLM-generated HTML, and content travels in GetTree for every item — so a
// bench with empty content would measure a tree the product never produces.
const SCALES = [
  { totalItems: 100, contentBytes: 2048, depth: 3, branching: 5 },
  { totalItems: 1_000, contentBytes: 2048, depth: 4, branching: 6 },
  { totalItems: 5_000, contentBytes: 2048, depth: 5, branching: 7 },
  { totalItems: 20_000, contentBytes: 2048, depth: 6, branching: 8 },
];

const WORKSPACE_ID = 'bench_ws';

function stubWorkspacePaper(workspaceId: string): Paper {
  return { id: workspaceId, title: workspaceId, description: '', content: '', childIds: [], parentId: null };
}

for (const scale of SCALES) {
  const { items, resolved } = buildMockTree(WORKSPACE_ID, { ...scale, seed: 7 });
  const label = `${resolved.totalItems} items x ${resolved.contentBytes}B`;

  const treeMessage = create(TreeSchema, { workspaceId: WORKSPACE_ID, items });
  const responseJson = toJson(GetTreeResponseSchema, create(GetTreeResponseSchema, { tree: treeMessage }));
  // Byte length, not string length: content is largely Japanese, and
  // String.prototype.length counts UTF-16 code units — it under-reports a
  // 3-byte character as 1. Gzip is reported too because that is what actually
  // crosses the network; the uncompressed size is what the JSON parser walks.
  const jsonText = JSON.stringify(responseJson);
  const wireBytes = Buffer.byteLength(jsonText, 'utf8');
  const gzipBytes = gzipSync(Buffer.from(jsonText, 'utf8')).length;
  const mib = (bytes: number) => (bytes / 1_048_576).toFixed(2);

  // Projection is pure over an already-built cache, so build the cache once —
  // otherwise the row would silently measure replaceWorkspaceTree as well.
  const projectionCache = createWorkspaceTreeCache();
  const { rootNodeIds } = projectionCache.replaceWorkspaceTree(WORKSPACE_ID, items);
  const projectionItems = projectionCache.getTreeItems(WORKSPACE_ID);

  describe(`${label} (${mib(wireBytes)} MiB JSON, ${mib(gzipBytes)} MiB gzipped)`, () => {
    bench('decode GetTree response (JSON codec)', () => {
      fromJson(GetTreeResponseSchema, responseJson);
    });

    bench('replaceWorkspaceTree', () => {
      createWorkspaceTreeCache().replaceWorkspaceTree(WORKSPACE_ID, items);
    });

    bench('projectWorkspacePapers (runs on every expansion)', () => {
      projectWorkspacePapers(WORKSPACE_ID, projectionItems, rootNodeIds, stubWorkspacePaper);
    });
  });
}

// A single expansion in isolation: merge one subtree's worth of items, then
// re-project. loadSubtreeAndProject does exactly this, so the cost of opening
// one paper in an N-item workspace is (small merge + full re-projection).
describe('one expansion in a large workspace', () => {
  const { items } = buildMockTree(WORKSPACE_ID, { totalItems: 5_000, depth: 5, branching: 7, contentBytes: 2048, seed: 7 });
  const cache = createWorkspaceTreeCache();
  const { rootNodeIds } = cache.replaceWorkspaceTree(WORKSPACE_ID, items);
  const treeItems = cache.getTreeItems(WORKSPACE_ID);
  const subtreeBatch = Array.from(treeItems.values()).slice(0, 7);

  bench('mergeSubtreeItems (7 items)', () => {
    cache.mergeSubtreeItems(WORKSPACE_ID, subtreeBatch);
  });

  bench('mergeSubtreeItems + projectWorkspacePapers (5000 items)', () => {
    cache.mergeSubtreeItems(WORKSPACE_ID, subtreeBatch);
    projectWorkspacePapers(WORKSPACE_ID, treeItems, rootNodeIds, stubWorkspacePaper);
  });
});

// What the two candidate fixes would actually buy, measured rather than
// assumed. Both are compared against the same 5,000-item tree:
//
//   codec        — connect-web on the binary codec instead of JSON
//                  (`useBinaryFormat: true` in lib/connect.ts)
//   content lazy — GetTree returning content only for root nodes, with each
//                  node's body fetched when its paper opens
//
// Sizes are UTF-8 bytes and gzip, because the transfer is compressed but the
// parser walks the uncompressed form.
describe('candidate fixes (5000 items x 2048B)', () => {
  const { items } = buildMockTree(WORKSPACE_ID, { totalItems: 5_000, depth: 5, branching: 7, contentBytes: 2048, seed: 7 });
  const lean = items.map((item) => create(ItemSchema, {
    ...item,
    content: item.parentId === '' ? item.content : '',
    overrideCss: item.parentId === '' ? item.overrideCss : '',
  }));

  for (const [label, list] of [['full', items], ['content lazy', lean]] as const) {
    const response = create(GetTreeResponseSchema, {
      tree: create(TreeSchema, { workspaceId: WORKSPACE_ID, items: list }),
    });
    const json = toJson(GetTreeResponseSchema, response);
    const jsonText = JSON.stringify(json);
    const binary = toBinary(GetTreeResponseSchema, response);
    const size = (bytes: number) => `${(bytes / 1_048_576).toFixed(2)} MiB`;
    console.info(`[${label}] json ${size(Buffer.byteLength(jsonText, 'utf8'))} (gzip ${size(gzipSync(Buffer.from(jsonText, 'utf8')).length)})`
      + ` | binary ${size(binary.length)} (gzip ${size(gzipSync(Buffer.from(binary)).length)})`);

    bench(`${label}: decode JSON`, () => {
      fromJson(GetTreeResponseSchema, json);
    });
    bench(`${label}: decode binary`, () => {
      fromBinary(GetTreeResponseSchema, binary);
    });
  }
});
