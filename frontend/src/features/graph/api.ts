import { callRPC } from '@/shared/lib/api';

export interface ApiNode {
  id: string;
  canonical_node_id?: string;
  scope: 'document' | 'canonical';
  label: string;
  level: number;
  category: 'concept' | 'entity' | 'claim' | 'evidence' | 'counter';
  entity_type?: string;
  description: string;
  summary_html?: string;
}

export interface ApiEdge {
  id: string;
  source: string;
  target: string;
  type: string;
  scope: 'document' | 'canonical';
}

export interface Graph {
  nodes: ApiNode[];
  edges: ApiEdge[];
}

export interface EntityRef {
  workspace_id: string;
  scope: 'document' | 'canonical';
  id: string;
  document_id?: string;
}

export interface GraphEntityDetail {
  ref: EntityRef;
  node: ApiNode;
  related_edges: ApiEdge[];
  evidence: {
    source_chunks: Array<{
      chunk_id: string;
      document_id: string;
      heading: string;
      text: string;
      source_page?: number;
    }>;
    source_document_ids: string[];
  };
  representative_nodes: ApiNode[];
}

export async function getGraph(
  workspaceId: string,
  documentId: string,
  opts: { levelFilters?: number[]; categoryFilters?: string[] } = {},
): Promise<Graph> {
  const res = await callRPC<
    { workspace_id: string; document_id: string; level_filters?: number[]; category_filters?: string[] },
    { graph: Graph }
  >('GraphService', 'GetGraph', {
    workspace_id: workspaceId,
    document_id: documentId,
    level_filters: opts.levelFilters,
    category_filters: opts.categoryFilters,
  });
  return res.graph;
}

export async function getGraphEntityDetail(
  targetRef: EntityRef,
  resolveAliases = false,
): Promise<GraphEntityDetail> {
  const res = await callRPC<
    { target_ref: EntityRef; resolve_aliases: boolean },
    { detail: GraphEntityDetail }
  >('NodeService', 'GetGraphEntityDetail', {
    target_ref: targetRef,
    resolve_aliases: resolveAliases,
  });
  return res.detail;
}

export async function recordNodeView(
  workspaceId: string,
  nodeId: string,
  documentId: string,
): Promise<void> {
  await callRPC('NodeService', 'RecordNodeView', {
    workspace_id: workspaceId,
    node_id: nodeId,
    document_id: documentId,
  });
}
