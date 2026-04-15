import { callRPC } from '@/lib/rpc';

export interface ApiNode {
  id: string;
  canonical_node_id?: string;
  scope: 'document' | 'canonical';
  label: string;
  level: number;
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

interface ConnectNode {
  id: string;
  canonicalNodeId?: string;
  scope: string;
  label: string;
  level: number;
  entityType?: string;
  description: string;
  summaryHtml?: string;
  documentId?: string;
}

interface ConnectEdge {
  id: string;
  source: string;
  target: string;
  type: string;
  scope: string;
}

interface ConnectGraph {
  nodes: ConnectNode[];
  edges: ConnectEdge[];
}

function mapScope(scope: string): ApiNode['scope'] {
  return scope === 'GRAPH_PROJECTION_SCOPE_CANONICAL' ? 'canonical' : 'document';
}

function mapEntityType(entityType?: string): string | undefined {
  switch (entityType) {
    case 'NODE_ENTITY_TYPE_ORGANIZATION':
      return 'organization';
    case 'NODE_ENTITY_TYPE_PERSON':
      return 'person';
    case 'NODE_ENTITY_TYPE_METRIC':
      return 'metric';
    case 'NODE_ENTITY_TYPE_DATE':
      return 'date';
    case 'NODE_ENTITY_TYPE_LOCATION':
      return 'location';
    default:
      return undefined;
  }
}

function mapNode(node: ConnectNode): ApiNode {
  return {
    id: node.id,
    canonical_node_id: node.canonicalNodeId,
    scope: mapScope(node.scope),
    label: node.label,
    level: node.level,
    entity_type: mapEntityType(node.entityType),
    description: node.description,
    summary_html: node.summaryHtml,
  };
}

function mapEdge(edge: ConnectEdge): ApiEdge {
  return {
    id: edge.id,
    source: edge.source,
    target: edge.target,
    type: edge.type,
    scope: edge.scope === 'GRAPH_PROJECTION_SCOPE_CANONICAL' ? 'canonical' : 'document',
  };
}

function mapGraph(graph: ConnectGraph): Graph {
  return {
    nodes: (graph.nodes ?? []).map(mapNode),
    edges: (graph.edges ?? []).map(mapEdge),
  };
}

function scopeToProto(scope: EntityRef['scope']): string {
  return scope === 'canonical' ? 'GRAPH_PROJECTION_SCOPE_CANONICAL' : 'GRAPH_PROJECTION_SCOPE_DOCUMENT';
}

export async function getGraph(
  workspaceId: string,
  opts: { levelFilters?: number[] } = {},
): Promise<Graph> {
  const res = await callRPC<
    { workspaceId: string; levelFilters?: number[] },
    { graph: ConnectGraph }
  >('GraphService', 'GetGraph', {
    workspaceId,
    levelFilters: opts.levelFilters,
  });
  return mapGraph(res.graph);
}

export async function getGraphEntityDetail(
  targetRef: EntityRef,
  resolveAliases = false,
): Promise<GraphEntityDetail> {
  const res = await callRPC<
    {
      targetRef: { workspaceId: string; scope: string; id: string; documentId?: string };
      resolveAliases: boolean;
    },
    {
      detail: {
        ref: { workspaceId: string; scope: string; id: string; documentId?: string };
        node: ConnectNode;
        relatedEdges: ConnectEdge[];
        evidence: {
          sourceChunks: Array<{
            chunkId: string;
            documentId: string;
            text: string;
            sourcePage?: number;
          }>;
          sourceDocumentIds: string[];
        };
        representativeNodes: ConnectNode[];
      };
    }
  >('NodeService', 'GetGraphEntityDetail', {
    targetRef: {
      workspaceId: targetRef.workspace_id,
      scope: scopeToProto(targetRef.scope),
      id: targetRef.id,
      documentId: targetRef.document_id,
    },
    resolveAliases,
  });
  return {
    ref: {
      workspace_id: res.detail.ref.workspaceId,
      scope: res.detail.ref.scope === 'GRAPH_PROJECTION_SCOPE_CANONICAL' ? 'canonical' : 'document',
      id: res.detail.ref.id,
      document_id: res.detail.ref.documentId,
    },
    node: mapNode(res.detail.node),
    related_edges: (res.detail.relatedEdges ?? []).map(mapEdge),
    evidence: {
      source_chunks: (res.detail.evidence?.sourceChunks ?? []).map((chunk) => ({
        chunk_id: chunk.chunkId,
        document_id: chunk.documentId,
        heading: '',
        text: chunk.text,
        source_page: chunk.sourcePage,
      })),
      source_document_ids: res.detail.evidence?.sourceDocumentIds ?? [],
    },
    representative_nodes: (res.detail.representativeNodes ?? []).map(mapNode),
  };
}
