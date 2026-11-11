import { z } from "zod";

/**
 * Kind of flow node representing different architectural or semantic elements.
 */
export const flowNodeKindSchema = z.enum(["room", "process", "decision", "data", "external"]);

/**
 * Schema for a single node within a flow graph.
 */
export const flowNodeSchema = z.object({
  id: z.string().regex(/^[a-z][a-z0-9-]{0,39}$/),
  title: z.string().trim().min(1).max(60),
  summary: z.string().trim().min(1).max(240),
  kind: flowNodeKindSchema,
  parentId: z.string().regex(/^[a-z][a-z0-9-]{0,39}$/).nullable(),
  evidence: z.array(z.string().trim().min(1).max(240)).max(4),
});

/**
 * Schema for a directed edge connecting two nodes in a flow graph.
 */
export const flowEdgeSchema = z.object({
  source: z.string().regex(/^[a-z][a-z0-9-]{0,39}$/),
  target: z.string().regex(/^[a-z][a-z0-9-]{0,39}$/),
  label: z.string().trim().max(60),
});

/**
 * Schema validating the entire flow graph structure including node uniqueness,
 * parent reference validity, and edge endpoints.
 */
export const flowGraphSchema = z.object({
  title: z.string().trim().min(1).max(80),
  summary: z.string().trim().min(1).max(400),
  nodes: z.array(flowNodeSchema).min(1),
  edges: z.array(flowEdgeSchema),
}).superRefine((graph, context) => {
  const ids = new Set<string>();
  graph.nodes.forEach((node, index) => {
    if (ids.has(node.id)) {
      context.addIssue({ code: "custom", path: ["nodes", index, "id"], message: `Duplicate node id: ${node.id}` });
    }
    ids.add(node.id);
  });
  graph.nodes.forEach((node, index) => {
    if (node.parentId && !ids.has(node.parentId)) {
      context.addIssue({ code: "custom", path: ["nodes", index, "parentId"], message: `Unknown parent: ${node.parentId}` });
    }
    if (node.parentId === node.id) {
      context.addIssue({ code: "custom", path: ["nodes", index, "parentId"], message: "A node cannot parent itself" });
    }
  });
  graph.edges.forEach((edge, index) => {
    if (!ids.has(edge.source) || !ids.has(edge.target)) {
      context.addIssue({ code: "custom", path: ["edges", index], message: "Edge references an unknown node" });
    }
    if (edge.source === edge.target) {
      context.addIssue({ code: "custom", path: ["edges", index], message: "Self edges are not allowed" });
    }
  });
});

/** Semantic categorization of a flow node. */
export type FlowNodeKind = z.infer<typeof flowNodeKindSchema>;
/** A single node representing a step, decision, data store, room, or external system. */
export type FlowNode = z.infer<typeof flowNodeSchema>;
/** A directed relationship between two nodes in the flow graph. */
export type FlowEdge = z.infer<typeof flowEdgeSchema>;
/** Complete hierarchical flow graph document. */
export type FlowGraph = z.infer<typeof flowGraphSchema>;

/**
 * Schema for a subgraph that can be appended to an existing room.
 *
 * @property nodes New nodes to validate and append.
 * @property edges Relationships between the new nodes and existing graph nodes.
 */
export const flowGraphExpansionSchema = z.object({
  nodes: z.array(flowNodeSchema).min(1),
  edges: z.array(flowEdgeSchema),
});
/** Subgraph expansion payload added to an existing room node. */
export type FlowGraphExpansion = z.infer<typeof flowGraphExpansionSchema>;

/**
 * Schema for normalized canvas coordinates.
 *
 * @property x Horizontal percentage in the inclusive 4..96 range.
 * @property y Vertical percentage in the inclusive 8..92 range.
 */
export const flowNodePositionSchema = z.object({
  x: z.number().min(4).max(96),
  y: z.number().min(8).max(92),
});
/** Maps each flow node ID to its normalized canvas position. */
export const graphLayoutSchema = z.record(z.string(), flowNodePositionSchema);
/** Normalized 2D coordinates (percentage values) for a node. */
export type FlowNodePosition = z.infer<typeof flowNodePositionSchema>;
/** Map of node IDs to normalized canvas positions. */
export type GraphLayout = z.infer<typeof graphLayoutSchema>;

/**
 * Schema for the persisted graph bundle written by the desktop repository.
 *
 * @property projectId UUID of the project that was analyzed.
 * @property provider Provider that generated the graph.
 * @property snapshotHash Hash of the source snapshot used as generation input.
 * @property generatedAt ISO-8601 generation timestamp.
 * @property graph Validated hierarchical flow graph.
 * @property layout Node positions keyed by node ID.
 * @property layoutVersion Optional layout algorithm version for future migrations.
 */
export const generatedFlowGraphSchema = z.object({
  projectId: z.string().uuid(),
  provider: z.enum(["codex", "antigravity-cli"]),
  snapshotHash: z.string().min(1).max(128),
  generatedAt: z.string().min(1).max(64),
  graph: flowGraphSchema,
  layout: graphLayoutSchema,
  layoutVersion: z.literal(2).optional(),
});
/** Persisted flow graph bundle with associated layout and generation metadata. */
export type GeneratedFlowGraph = z.infer<typeof generatedFlowGraphSchema>;

/**
 * A flow node augmented with canvas x/y coordinate percentages.
 *
 * @property x Horizontal canvas percentage.
 * @property y Vertical canvas percentage.
 */
export type PositionedFlowNode = FlowNode & { x: number; y: number };
/**
 * An edge projected onto a specific room scope, collapsing cross-boundary endpoints.
 *
 * @property source Visible source node ID, or null when the source is outside the scope.
 * @property target Visible target node ID, or null when the target is outside the scope.
 * @property sourceOutsideId Original source ID when the source was collapsed.
 * @property targetOutsideId Original target ID when the target was collapsed.
 * @property labels Labels from all edges represented by this projection.
 * @property count Number of original edges merged into this projection.
 */
export type ProjectedFlowEdge = {
  source: string | null;
  target: string | null;
  sourceOutsideId: string | null;
  targetOutsideId: string | null;
  labels: string[];
  count: number;
};
/**
 * Nodes and projected edges visible within a given parent room scope.
 *
 * @property nodes Direct child nodes visible in the scope.
 * @property edges Deduplicated edges after ancestor endpoints are projected.
 */
export type ScopeProjection = {
  nodes: FlowNode[];
  edges: ProjectedFlowEdge[];
};
/**
 * Boundary port endpoint connecting an internal node to an external ancestor/descendant.
 *
 * @property id External node identifier.
 * @property title External node display title.
 * @property label Relationship label shown on the port.
 */
export type BoundaryEndpoint = { id: string; title: string; label: string };
/**
 * Input or output connection port shown on a room's canvas perimeter.
 *
 * @property side Whether the port receives input or emits output.
 * @property nodeId Internal node represented by the port.
 * @property endpoints External connections grouped under the port.
 * @property count Number of original edges represented by the port.
 */
export type ScopeBoundaryPort = {
  side: "input" | "output";
  nodeId: string;
  endpoints: BoundaryEndpoint[];
  count: number;
};
/**
 * Node placed in a portal preview miniature.
 *
 * @property isEntry Whether an edge enters this node from outside the preview.
 * @property isExit Whether an edge leaves this node toward outside the preview.
 */
export type PortalPreviewNode = PositionedFlowNode & { isEntry: boolean; isExit: boolean };
/**
 * Miniature overview of nodes and edges contained within a folded room portal.
 *
 * @property nodes Positioned nodes selected for the miniature.
 * @property edges Internal connections between preview nodes.
 * @property childCount Number of direct children of the folded room.
 * @property descendantCount Total descendants below the folded room.
 * @property hiddenCount Descendants omitted because of the preview limit.
 */
export type PortalPreview = {
  nodes: PortalPreviewNode[];
  edges: Array<{ source: string; target: string }>;
  childCount: number;
  descendantCount: number;
  hiddenCount: number;
};

/** Maximum nodes allowed in a single room before balancing splits it into continuation rooms. */
export const FLOWFOLD_ROOM_MAX_NODES = 7;
/** Maximum nodes rendered inside a miniature portal preview. */
export const PORTAL_PREVIEW_MAX_NODES = 5;

export const FLOW_GRAPH_JSON_SCHEMA = {
  type: "object",
  additionalProperties: false,
  required: ["title", "summary", "nodes", "edges"],
  properties: {
    title: { type: "string", minLength: 1, maxLength: 80 },
    summary: { type: "string", minLength: 1, maxLength: 400 },
    nodes: {
      type: "array",
      minItems: 1,
      items: {
        type: "object",
        additionalProperties: false,
        required: ["id", "title", "summary", "kind", "parentId", "evidence"],
        properties: {
          id: { type: "string", pattern: "^[a-z][a-z0-9-]{0,39}$" },
          title: { type: "string", minLength: 1, maxLength: 60 },
          summary: { type: "string", minLength: 1, maxLength: 240 },
          kind: { type: "string", enum: ["room", "process", "decision", "data", "external"] },
          parentId: { anyOf: [{ type: "string", pattern: "^[a-z][a-z0-9-]{0,39}$" }, { type: "null" }] },
          evidence: { type: "array", maxItems: 4, items: { type: "string", minLength: 1, maxLength: 240 } },
        },
      },
    },
    edges: {
      type: "array",
      items: {
        type: "object",
        additionalProperties: false,
        required: ["source", "target", "label"],
        properties: {
          source: { type: "string", pattern: "^[a-z][a-z0-9-]{0,39}$" },
          target: { type: "string", pattern: "^[a-z][a-z0-9-]{0,39}$" },
          label: { type: "string", maxLength: 60 },
        },
      },
    },
  },
} as const;

export const FLOW_GRAPH_EXPANSION_JSON_SCHEMA = {
  type: "object",
  additionalProperties: false,
  required: ["nodes", "edges"],
  properties: {
    nodes: {
      type: "array",
      minItems: 1,
      items: FLOW_GRAPH_JSON_SCHEMA.properties.nodes.items,
    },
    edges: {
      type: "array",
      items: FLOW_GRAPH_JSON_SCHEMA.properties.edges.items,
    },
  },
} as const;

/**
 * Validates and parses an unknown object into a valid FlowGraph.
 * @param value - The input value to parse.
 * @returns The parsed FlowGraph.
 */
export function parseFlowGraph(value: unknown): FlowGraph {
  return flowGraphSchema.parse(value);
}

/**
 * Validates and parses an unknown object into a valid GeneratedFlowGraph.
 * @param value - The input value to parse.
 * @returns The parsed GeneratedFlowGraph.
 */
export function parseGeneratedFlowGraph(value: unknown): GeneratedFlowGraph {
  return generatedFlowGraphSchema.parse(value);
}

/**
 * Parses raw JSON or markdown-fenced text into a validated FlowGraph.
 * @param text - The raw text containing JSON or markdown code block.
 * @returns The parsed FlowGraph.
 */
export function parseFlowGraphText(text: string): FlowGraph {
  return parseFlowGraph(JSON.parse(unfenceJson(text)));
}

/**
 * Validates and parses an unknown object into a valid FlowGraphExpansion.
 * @param value - The input value to parse.
 * @returns The parsed FlowGraphExpansion.
 */
export function parseFlowGraphExpansion(value: unknown): FlowGraphExpansion {
  return flowGraphExpansionSchema.parse(value);
}

/**
 * Parses raw JSON or markdown-fenced text into a validated FlowGraphExpansion.
 * @param text - The raw text containing JSON or markdown code block.
 * @returns The parsed FlowGraphExpansion.
 */
export function parseFlowGraphExpansionText(text: string): FlowGraphExpansion {
  return parseFlowGraphExpansion(JSON.parse(unfenceJson(text)));
}

/**
 * Computes topological 2D layout positions for the root-level (parentId === null) nodes of a graph.
 * @param graph - The flow graph to lay out.
 * @returns Array of positioned flow nodes.
 */
export function layoutRootNodes(graph: FlowGraph): PositionedFlowNode[] {
  const roots = graph.nodes.filter((node) => node.parentId === null);
  const visible = roots.length > 0 ? roots : graph.nodes;
  return layoutFlowNodes(visible, graph.edges);
}

/**
 * Computes topologically arranged 2D percentage coordinates for a set of visible nodes and edges.
 * Handles acyclic flows with topological sorting and clusters cyclical components.
 * @param visible - The nodes to position.
 * @param edges - The edges connecting the visible nodes.
 * @returns Array of positioned flow nodes with x and y coordinates (4-96 range).
 */
export function layoutFlowNodes(visible: FlowNode[], edges: FlowEdge[] = []): PositionedFlowNode[] {
  if (visible.length === 0) return [];
  const ids = new Set(visible.map((node) => node.id));
  const order = new Map(visible.map((node, index) => [node.id, index]));
  const outgoing = new Map(visible.map((node) => [node.id, [] as string[]]));
  const indegree = new Map(visible.map((node) => [node.id, 0]));
  for (const edge of edges) {
    if (!ids.has(edge.source) || !ids.has(edge.target) || edge.source === edge.target) continue;
    if (outgoing.get(edge.source)!.includes(edge.target)) continue;
    outgoing.get(edge.source)!.push(edge.target);
    indegree.set(edge.target, indegree.get(edge.target)! + 1);
  }

  const ranks = new Map(visible.map((node) => [node.id, 0]));
  const queue = visible.filter((node) => indegree.get(node.id) === 0).map((node) => node.id);
  const visited = new Set<string>();
  while (queue.length) {
    queue.sort((left, right) => order.get(left)! - order.get(right)!);
    const source = queue.shift()!;
    visited.add(source);
    for (const target of outgoing.get(source)!) {
      ranks.set(target, Math.max(ranks.get(target)!, ranks.get(source)! + 1));
      indegree.set(target, indegree.get(target)! - 1);
      if (indegree.get(target) === 0) queue.push(target);
    }
  }

  // A cycle has no topological start. Keep its members together after the acyclic flow.
  const highestRank = Math.max(0, ...ranks.values());
  visible.filter((node) => !visited.has(node.id)).forEach((node, index) => {
    ranks.set(node.id, highestRank + Math.floor(index / 3) + (visited.size ? 1 : 0));
  });

  const groups = new Map<number, FlowNode[]>();
  for (const node of visible) {
    const rank = ranks.get(node.id)!;
    const group = groups.get(rank) ?? [];
    group.push(node);
    groups.set(rank, group);
  }
  const orderedRanks = [...groups.keys()].sort((left, right) => left - right);
  const rankIndex = new Map(orderedRanks.map((rank, index) => [rank, index]));
  const result = new Map<string, PositionedFlowNode>();
  for (const [rank, nodes] of groups) {
    const column = rankIndex.get(rank)!;
    const x = orderedRanks.length === 1 ? 50 : 15 + (70 * column) / (orderedRanks.length - 1);
    nodes.forEach((node, row) => {
      const y = nodes.length === 1 ? 50 : 20 + (60 * row) / (nodes.length - 1);
      result.set(node.id, { ...node, x, y });
    });
  }
  return visible.map((node) => result.get(node.id)!);
}

/**
 * Creates default layout coordinates for all nodes in the graph across all scope rooms.
 * Preserves existing node positions if already present in the existing layout map.
 * @param graph - The flow graph to lay out.
 * @param existing - Optional map of pre-existing node positions to preserve.
 * @returns Complete GraphLayout mapping node IDs to coordinates.
 */
export function createDefaultGraphLayout(graph: FlowGraph, existing: GraphLayout = {}): GraphLayout {
  const layout: GraphLayout = { ...existing };
  const parentIds = new Set<string | null>([null, ...graph.nodes.map((node) => node.parentId)]);
  for (const parentId of parentIds) {
    const nodes = graph.nodes.filter((node) => node.parentId === parentId);
    const projection = projectFlowToScope(graph, parentId);
    const projectedEdges = projection.edges
      .filter((edge): edge is ProjectedFlowEdge & { source: string; target: string } => edge.source !== null && edge.target !== null)
      .map((edge) => ({ source: edge.source, target: edge.target, label: edge.labels[0] ?? "" }));
    for (const node of layoutFlowNodes(nodes, projectedEdges)) {
      if (!layout[node.id]) layout[node.id] = { x: node.x, y: node.y };
    }
  }
  return layout;
}

// A Room shows one generation of nodes, but an edge may start or end many levels
// deeper. Both are answered by the same question: which node of this Room, if
// any, does an arbitrary node belong to?
/**
 * Resolves which direct child node of scopeId an arbitrary node belongs to by traversing parentId links.
 * @param graph - The flow graph.
 * @param scopeId - The room scope ID (or null for root scope).
 * @returns Function mapping any nodeId to its ancestor representative in scopeId.
 */
function scopeRepresentative(graph: FlowGraph, scopeId: string | null): (nodeId: string) => string | null {
  const byId = new Map(graph.nodes.map((node) => [node.id, node]));
  return (nodeId: string): string | null => {
    let cursor = byId.get(nodeId);
    const visited = new Set<string>();
    while (cursor && !visited.has(cursor.id)) {
      if (cursor.parentId === scopeId) return cursor.id;
      visited.add(cursor.id);
      cursor = cursor.parentId ? byId.get(cursor.parentId) : undefined;
    }
    return null;
  };
}

/**
 * Projects graph nodes and edges to a specific room scope, collapsing cross-boundary endpoints.
 * @param graph - The flow graph.
 * @param scopeId - The target room scope ID (or null for root).
 * @returns ScopeProjection containing direct child nodes and projected boundary edges.
 */
export function projectFlowToScope(graph: FlowGraph, scopeId: string | null): ScopeProjection {
  const nodes = graph.nodes.filter((node) => node.parentId === scopeId);
  const representative = scopeRepresentative(graph, scopeId);

  const projected = new Map<string, ProjectedFlowEdge>();
  for (const edge of graph.edges) {
    const source = representative(edge.source);
    const target = representative(edge.target);
    if (source === target) continue;
    if (source === null && target === null) continue;
    const key = `${source ?? "outside"}\u0000${target ?? "outside"}`;
    const current = projected.get(key);
    if (current) {
      current.count += 1;
      if (edge.label && !current.labels.includes(edge.label)) current.labels.push(edge.label);
      continue;
    }
    projected.set(key, {
      source,
      target,
      sourceOutsideId: source === null ? edge.source : null,
      targetOutsideId: target === null ? edge.target : null,
      labels: edge.label ? [edge.label] : [],
      count: 1,
    });
  }
  return { nodes, edges: [...projected.values()] };
}

/**
 * Rebalances a flow graph by breaking oversized rooms into continuation rooms ("Continued flow")
 * so that no individual room exceeds the maximum node limit.
 * @param graph - The flow graph to balance.
 * @param maximum - Maximum number of nodes per room scope.
 * @returns Balanced FlowGraph.
 */
export function balanceFlowGraphScopes(graph: FlowGraph, maximum = FLOWFOLD_ROOM_MAX_NODES): FlowGraph {
  if (maximum < 2) throw new Error("A FlowFold Room must allow at least two nodes");
  let nodes = graph.nodes.map((node) => ({ ...node }));
  const usedIds = new Set(nodes.map((node) => node.id));
  const queue: Array<string | null> = [null, ...nodes.map((node) => node.id)];
  for (let index = 0; index < queue.length; index += 1) {
    const parentId = queue[index];
    const children = nodes.filter((node) => node.parentId === parentId);
    if (children.length <= maximum) continue;
    const overflow = children.slice(maximum - 1);
    const baseId = parentId ? `${parentId}-continued` : "continued-flow";
    const roomId = uniqueNodeId(baseId, usedIds);
    usedIds.add(roomId);
    const room: FlowNode = {
      id: roomId,
      title: "Continued flow",
      summary: `Groups ${overflow.length} additional steps so this Room stays readable.`,
      kind: "room",
      parentId,
      evidence: [...new Set(overflow.flatMap((node) => node.evidence))].slice(0, 4),
    };
    nodes = [...nodes.map((node) => overflow.some((item) => item.id === node.id) ? { ...node, parentId: roomId } : node), room];
    queue.push(roomId);
    index -= 1;
  }
  return parseFlowGraph({ ...graph, nodes });
}

// Entering a Room must not cut the flow. Edges that leave the Room stay visible
// as a port on the boundary, named after the node on the other side.
/**
 * Calculates perimeter boundary ports representing external cross-boundary inputs and outputs for a room.
 * @param graph - The flow graph.
 * @param scopeId - Target room scope ID.
 * @returns Array of ScopeBoundaryPort instances.
 */
export function scopeBoundaryPorts(graph: FlowGraph, scopeId: string | null): ScopeBoundaryPort[] {
  const representative = scopeRepresentative(graph, scopeId);
  const byId = new Map(graph.nodes.map((node) => [node.id, node]));
  const ports = new Map<string, ScopeBoundaryPort>();
  const add = (side: "input" | "output", nodeId: string, outsideId: string, label: string) => {
    const key = `${side}\u0000${nodeId}`;
    const port = ports.get(key) ?? { side, nodeId, endpoints: [], count: 0 };
    port.count += 1;
    if (!port.endpoints.some((endpoint) => endpoint.id === outsideId)) {
      port.endpoints.push({ id: outsideId, title: byId.get(outsideId)?.title ?? outsideId, label });
    }
    ports.set(key, port);
  };
  const inside = graph.nodes.filter((node) => node.parentId === scopeId).map((node) => node.id);
  const fed = new Set<string>();
  const feeding = new Set<string>();
  for (const edge of graph.edges) {
    const source = representative(edge.source);
    const target = representative(edge.target);
    if (source !== null && target !== null && source !== target) { fed.add(target); feeding.add(source); }
  }
  const entries = inside.filter((id) => !fed.has(id));
  const exits = inside.filter((id) => !feeding.has(id));
  const orFirst = (candidates: string[]) => (candidates.length > 0 ? candidates : inside.slice(0, 1));

  for (const edge of graph.edges) {
    const source = representative(edge.source);
    const target = representative(edge.target);
    if (source === null && target !== null) add("input", target, edge.source, edge.label);
    else if (target === null && source !== null) add("output", source, edge.target, edge.label);
    else if (source === null && target === null && scopeId !== null) {
      // The Room node itself is the endpoint. Hand that connection to the nodes
      // that open and close the flow inside, so the order survives the descent.
      if (edge.target === scopeId) for (const id of orFirst(entries)) add("input", id, edge.source, edge.label);
      if (edge.source === scopeId) for (const id of orFirst(exits)) add("output", id, edge.target, edge.label);
    }
  }
  return [...ports.values()];
}

// A Portal is a folded sheet of paper, not a link: it shows a miniature of the
// flow inside it. This is a summary snapshot, never a live child canvas.
/**
 * Constructs a miniature preview layout of nodes and edges contained inside a folded room portal.
 * @param graph - The flow graph.
 * @param nodeId - The room node ID to preview.
 * @param limit - Maximum number of preview nodes to include in miniature.
 * @returns PortalPreview model with positioned preview nodes and metrics.
 */
export function buildPortalPreview(graph: FlowGraph, nodeId: string, limit = PORTAL_PREVIEW_MAX_NODES): PortalPreview {
  const children = graph.nodes.filter((node) => node.parentId === nodeId);
  const descendantCount = descendantCountOf(graph, nodeId);
  if (children.length === 0) {
    return { nodes: [], edges: [], childCount: 0, descendantCount, hiddenCount: 0 };
  }
  const projection = projectFlowToScope(graph, nodeId);
  const inside = projection.edges.filter(
    (edge): edge is ProjectedFlowEdge & { source: string; target: string } => edge.source !== null && edge.target !== null,
  );
  const positioned = layoutFlowNodes(children, inside.map((edge) => ({ source: edge.source, target: edge.target, label: "" })));
  const entries = new Set(projection.edges.filter((edge) => edge.source === null && edge.target).map((edge) => edge.target!));
  const exits = new Set(projection.edges.filter((edge) => edge.target === null && edge.source).map((edge) => edge.source!));

  // Reading order first, so the miniature keeps the shape of the real flow when
  // it has to drop nodes.
  const ordered = [...positioned].sort((left, right) => left.x - right.x || left.y - right.y);
  const selected = ordered.slice(0, Math.max(1, limit));
  const selectedIds = new Set(selected.map((node) => node.id));
  const xs = selected.map((node) => node.x);
  const ys = selected.map((node) => node.y);
  const span = (values: number[]) => Math.max(...values) - Math.min(...values);
  const spanX = span(xs);
  const spanY = span(ys);
  const nodes = selected.map((node) => ({
    ...node,
    x: spanX < 1 ? 50 : 12 + ((node.x - Math.min(...xs)) / spanX) * 76,
    y: spanY < 1 ? 50 : 18 + ((node.y - Math.min(...ys)) / spanY) * 64,
    isEntry: entries.has(node.id),
    isExit: exits.has(node.id),
  }));
  return {
    nodes,
    edges: inside
      .filter((edge) => selectedIds.has(edge.source) && selectedIds.has(edge.target))
      .map((edge) => ({ source: edge.source, target: edge.target })),
    childCount: children.length,
    descendantCount,
    hiddenCount: children.length - selected.length,
  };
}

/**
 * Counts total recursive descendant nodes rooted under the given parent node ID.
 * @param graph - The flow graph.
 * @param nodeId - The root node ID.
 * @returns Total count of descendants.
 */
function descendantCountOf(graph: FlowGraph, nodeId: string): number {
  const found = new Set([nodeId]);
  let changed = true;
  while (changed) {
    changed = false;
    for (const node of graph.nodes) {
      if (node.parentId && found.has(node.parentId) && !found.has(node.id)) {
        found.add(node.id);
        changed = true;
      }
    }
  }
  return found.size - 1;
}

/**
 * Validates that an expanded flow graph strictly extends the current graph within scopeNodeId
 * without modifying or deleting existing nodes or edges outside the target room.
 * @param current - The baseline flow graph.
 * @param expanded - The proposed expanded graph.
 * @param scopeNodeId - The room node ID targeted for expansion.
 * @returns The validated expanded FlowGraph.
 */
export function validateScopeExpansion(current: FlowGraph, expanded: FlowGraph, scopeNodeId: string): FlowGraph {
  if (!current.nodes.some((node) => node.id === scopeNodeId)) throw new Error("Expansion scope does not exist");
  const expandedById = new Map(expanded.nodes.map((node) => [node.id, node]));
  for (const node of current.nodes) {
    const candidate = expandedById.get(node.id);
    if (!candidate || JSON.stringify(candidate) !== JSON.stringify(node)) {
      throw new Error(`Expansion changed existing node: ${node.id}`);
    }
  }
  for (const edge of current.edges) {
    if (!expanded.edges.some((candidate) => JSON.stringify(candidate) === JSON.stringify(edge))) {
      throw new Error(`Expansion removed or changed an existing edge: ${edge.source} -> ${edge.target}`);
    }
  }
  const currentIds = new Set(current.nodes.map((node) => node.id));
  const newNodes = expanded.nodes.filter((node) => !currentIds.has(node.id));
  const newIds = new Set(newNodes.map((node) => node.id));
  if (!newNodes.some((node) => node.parentId === scopeNodeId)) throw new Error("Expansion did not add a direct child to the Room");
  for (const node of newNodes) {
    let parentId = node.parentId;
    const visited = new Set([node.id]);
    while (parentId && !currentIds.has(parentId) && !visited.has(parentId)) {
      visited.add(parentId);
      parentId = expandedById.get(parentId)?.parentId ?? null;
    }
    if (parentId !== scopeNodeId) throw new Error(`Expansion added a node outside the requested Room: ${node.id}`);
  }
  for (const edge of expanded.edges) {
    const alreadyExisted = current.edges.some((candidate) => JSON.stringify(candidate) === JSON.stringify(edge));
    if (alreadyExisted) continue;
    if (!newIds.has(edge.source) && !newIds.has(edge.target)) {
      throw new Error(`Expansion added an unrelated edge between existing nodes: ${edge.source} -> ${edge.target}`);
    }
  }
  return expanded;
}

/**
 * Merges a validated FlowGraphExpansion payload into an existing flow graph under scopeNodeId.
 * @param current - The current baseline flow graph.
 * @param expansion - The expansion nodes and edges to append.
 * @param scopeNodeId - The parent room node ID receiving the expansion.
 * @returns The resulting merged FlowGraph.
 */
export function applyScopeExpansion(current: FlowGraph, expansion: FlowGraphExpansion, scopeNodeId: string): FlowGraph {
  if (!current.nodes.some((node) => node.id === scopeNodeId)) throw new Error("Expansion scope does not exist");
  const currentIds = new Set(current.nodes.map((node) => node.id));
  const newIds = new Set<string>();
  for (const node of expansion.nodes) {
    if (currentIds.has(node.id) || newIds.has(node.id)) throw new Error(`Expansion reused an existing node id: ${node.id}`);
    newIds.add(node.id);
  }
  const allById = new Map([...current.nodes, ...expansion.nodes].map((node) => [node.id, node]));
  if (!expansion.nodes.some((node) => node.parentId === scopeNodeId)) throw new Error("Expansion did not add a direct child to the Room");
  for (const node of expansion.nodes) {
    let parentId = node.parentId;
    const visited = new Set([node.id]);
    while (parentId && newIds.has(parentId) && !visited.has(parentId)) {
      visited.add(parentId);
      parentId = allById.get(parentId)?.parentId ?? null;
    }
    if (parentId !== scopeNodeId) throw new Error(`Expansion added a node outside the requested Room: ${node.id}`);
  }
  const allIds = new Set(allById.keys());
  for (const edge of expansion.edges) {
    if (!allIds.has(edge.source) || !allIds.has(edge.target)) throw new Error(`Expansion edge references an unknown node: ${edge.source} -> ${edge.target}`);
    if (edge.source === edge.target) throw new Error(`Expansion created a self edge: ${edge.source}`);
    if (!newIds.has(edge.source) && !newIds.has(edge.target)) throw new Error(`Expansion added an unrelated edge between existing nodes: ${edge.source} -> ${edge.target}`);
  }
  const existingEdgeKeys = new Set(current.edges.map(edgeKey));
  const newEdges = expansion.edges.filter((edge) => !existingEdgeKeys.has(edgeKey(edge)));
  return parseFlowGraph({ ...current, nodes: [...current.nodes, ...expansion.nodes], edges: [...current.edges, ...newEdges] });
}

/**
 * Creates a unique string key identifying a directed edge and its label.
 * @param edge - The flow edge.
 * @returns Unique string key.
 */
function edgeKey(edge: FlowEdge): string {
  return `${edge.source}\u0000${edge.target}\u0000${edge.label}`;
}

/**
 * Generates a unique, slugified node ID for a room, appending incrementing numbers if already used.
 * @param base - Base string for the ID.
 * @param used - Set of already allocated IDs.
 * @returns A unique node ID string.
 */
function uniqueNodeId(base: string, used: Set<string>): string {
  const normalized = base.toLowerCase().replace(/[^a-z0-9-]/g, "-").replace(/-+/g, "-").replace(/^-|-$/g, "").slice(0, 32) || "continued-flow";
  if (!used.has(normalized)) return normalized;
  for (let suffix = 2; suffix < 10_000; suffix += 1) {
    const candidate = `${normalized.slice(0, 39 - String(suffix).length)}-${suffix}`;
    if (!used.has(candidate)) return candidate;
  }
  throw new Error("Could not allocate a FlowFold continuation Room id");
}

/**
 * Strips markdown code blocks (e.g. ```json ... ```) from a text string.
 * @param text - Raw string possibly containing code block fences.
 * @returns The unfenced JSON string.
 */
function unfenceJson(text: string): string {
  const trimmed = text.trim();
  return trimmed.startsWith("```")
    ? trimmed.replace(/^```(?:json)?\s*/i, "").replace(/\s*```$/, "")
    : trimmed;
}
