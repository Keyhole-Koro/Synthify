import { buildPaperMap } from '@keyhole-koro/paper-in-paper';
import type { Paper, PaperMap } from '@keyhole-koro/paper-in-paper';
import type { ApiNode, ApiEdge } from './api';

/** ノードカテゴリ → ペーパーの色相 */
const CATEGORY_HUE: Record<string, number> = {
  concept: 220,  // 青
  entity: 140,   // 緑
  claim: 280,    // 紫
  evidence: 160, // 緑-シアン
  counter: 10,   // オレンジ-赤
};

/**
 * GetGraph レスポンスの nodes/edges から PaperMap を構築する。
 * hierarchical エッジがツリー構造を決定する。
 * 非階層エッジは summary_html 内の data-paper-id リンクとして既に埋め込まれている。
 */
export function buildPaperMapFromGraph(nodes: ApiNode[], edges: ApiEdge[]): PaperMap {
  const childMap = new Map<string, string[]>();
  const parentMap = new Map<string, string>();

  for (const edge of edges) {
    if (edge.type === 'hierarchical') {
      if (!childMap.has(edge.source)) {
        childMap.set(edge.source, []);
      }
      childMap.get(edge.source)!.push(edge.target);
      parentMap.set(edge.target, edge.source);
    }
  }

  const papers: Paper[] = nodes.map((node) => ({
    id: node.id,
    title: node.label,
    description: node.description,
    content: node.summary_html || `<p>${node.description}</p>`,
    hue: CATEGORY_HUE[node.category] ?? 220,
    parentId: parentMap.get(node.id) ?? null,
    childIds: childMap.get(node.id) ?? [],
  }));

  return buildPaperMap(papers);
}

/**
 * ツリー構造を持たない孤立ノード（親なし・子なし）の ID 一覧を返す。
 * paper-in-paper の unplacedNodeIds として使う。
 */
export function findUnplacedNodeIds(nodes: ApiNode[], edges: ApiEdge[]): string[] {
  const connectedByHierarchy = new Set<string>();
  for (const edge of edges) {
    if (edge.type === 'hierarchical') {
      connectedByHierarchy.add(edge.source);
      connectedByHierarchy.add(edge.target);
    }
  }
  // 複数のルート候補がある場合、level 0 以外を unplaced とする
  const roots = nodes.filter((n) => !connectedByHierarchy.has(n.id));
  return roots.filter((n) => n.level > 0).map((n) => n.id);
}

/** ルートノード ID（level 0 または親のないノード）を返す。 */
export function findRootNodeId(nodes: ApiNode[], edges: ApiEdge[]): string | undefined {
  const hasParent = new Set<string>();
  for (const edge of edges) {
    if (edge.type === 'hierarchical') {
      hasParent.add(edge.target);
    }
  }
  const root = nodes.find((n) => n.level === 0 && !hasParent.has(n.id));
  if (root) return root.id;
  // level 0 がなければ親のないノードを返す
  return nodes.find((n) => !hasParent.has(n.id))?.id;
}
