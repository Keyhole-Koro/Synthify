/**
 * Mock flow graph fixtures for standalone browser previewing and UI tests.
 *
 * @module
 */
import { createDefaultGraphLayout, parseFlowGraph, type FlowGraph, type GeneratedFlowGraph } from "@insightify/graph-domain";

/**
 * A hand-written Graph with cross-Room edges and nested Portals, so the preview
 * exercises boundary ports, edge bundling and Portal folds without an agent.
 */
const source = {
  title: "Insightify pipeline",
  summary: "How a repository becomes a navigable FlowFold graph.",
  nodes: [
    { id: "ingest", title: "Ingest repository", summary: "Watch the working tree and build a safe snapshot.", kind: "room", parentId: null, evidence: ["src/main/project-snapshot.ts"] },
    { id: "parse", title: "Parse sources", summary: "Turn files into a symbol graph the analyzer can walk.", kind: "room", parentId: null, evidence: ["src/main/parse/index.ts"] },
    { id: "analyze", title: "Analyze structure", summary: "Group symbols into Rooms and rank the primary flow.", kind: "room", parentId: null, evidence: ["packages/graph-domain/src/index.ts"] },
    { id: "render", title: "Render canvas", summary: "Lay out Portals and draw the flow for the current Room.", kind: "room", parentId: null, evidence: ["src/renderer/App.tsx"] },
    { id: "store", title: "Project store", summary: "Persist graph, layout and project identity locally.", kind: "data", parentId: null, evidence: ["src/main/project-repository.ts"] },

    { id: "watch-tree", title: "Watch tree", summary: "Observe file changes under the project root.", kind: "process", parentId: "ingest", evidence: ["src/main/watch.ts"] },
    { id: "read-files", title: "Read files", summary: "Read only text files inside the allow list.", kind: "process", parentId: "ingest", evidence: ["src/main/project-snapshot.ts"] },
    { id: "hash-snapshot", title: "Hash snapshot", summary: "Fingerprint the snapshot so stale graphs are detectable.", kind: "data", parentId: "ingest", evidence: ["src/main/project-snapshot.ts"] },

    { id: "tokenize", title: "Tokenize", summary: "Split each source file into tokens.", kind: "process", parentId: "parse", evidence: ["src/main/parse/tokenize.ts"] },
    { id: "build-ast", title: "Build AST", summary: "Assemble a tree per module.", kind: "process", parentId: "parse", evidence: ["src/main/parse/ast.ts"] },
    { id: "resolve-imports", title: "Resolve imports", summary: "Link modules into one dependency graph.", kind: "process", parentId: "parse", evidence: ["src/main/parse/resolve.ts"] },

    { id: "collect-symbols", title: "Collect symbols", summary: "Gather exported symbols and their call sites.", kind: "process", parentId: "analyze", evidence: ["src/main/analyze/symbols.ts"] },
    { id: "rank-flow", title: "Rank flow", summary: "Order nodes by edge direction, not by input order.", kind: "process", parentId: "analyze", evidence: ["packages/graph-domain/src/index.ts"] },
    { id: "group-rooms", title: "Group Rooms", summary: "Cut the graph into Rooms of one abstraction level.", kind: "process", parentId: "analyze", evidence: ["packages/graph-domain/src/index.ts"] },
    { id: "detect-cycles", title: "Detect cycles", summary: "Keep cyclic members together after the acyclic flow.", kind: "decision", parentId: "analyze", evidence: ["packages/graph-domain/src/index.ts"] },
    { id: "fold-overflow", title: "Fold overflow", summary: "Move nodes past seven into a continuation Room.", kind: "process", parentId: "analyze", evidence: ["packages/graph-domain/src/index.ts"] },

    { id: "rank-indegree", title: "Seed indegree", summary: "Find the nodes nothing points at.", kind: "process", parentId: "rank-flow", evidence: [] },
    { id: "rank-layers", title: "Assign layers", summary: "Push each target one column right of its source.", kind: "process", parentId: "rank-flow", evidence: [] },
    { id: "rank-rows", title: "Spread rows", summary: "Spread a column vertically around the centre line.", kind: "process", parentId: "rank-flow", evidence: [] },

    { id: "layout-stage", title: "Layout stage", summary: "Place Portals from the Room-local layout.", kind: "process", parentId: "render", evidence: ["src/renderer/App.tsx"] },
    { id: "draw-edges", title: "Draw edges", summary: "Route flow edges and boundary links.", kind: "process", parentId: "render", evidence: ["src/renderer/App.tsx"] },
    { id: "paint-portals", title: "Paint Portals", summary: "Render folds at the current semantic level.", kind: "process", parentId: "render", evidence: ["src/renderer/styles.css"] },
  ],
  edges: [
    { source: "watch-tree", target: "read-files", label: "changed paths" },
    { source: "read-files", target: "hash-snapshot", label: "file text" },
    { source: "hash-snapshot", target: "tokenize", label: "snapshot" },
    { source: "tokenize", target: "build-ast", label: "tokens" },
    { source: "build-ast", target: "resolve-imports", label: "modules" },
    { source: "resolve-imports", target: "collect-symbols", label: "module graph" },
    { source: "collect-symbols", target: "rank-flow", label: "symbols" },
    { source: "rank-flow", target: "group-rooms", label: "ranked nodes" },
    { source: "group-rooms", target: "detect-cycles", label: "rooms" },
    { source: "detect-cycles", target: "fold-overflow", label: "cycles kept" },
    { source: "rank-indegree", target: "rank-layers", label: "roots" },
    { source: "rank-layers", target: "rank-rows", label: "columns" },
    { source: "fold-overflow", target: "layout-stage", label: "balanced graph" },
    { source: "layout-stage", target: "draw-edges", label: "positions" },
    { source: "draw-edges", target: "paint-portals", label: "geometry" },
    { source: "group-rooms", target: "store", label: "graph" },
    { source: "paint-portals", target: "store", label: "layout" },
  ],
} satisfies FlowGraph;

/**
 * Pre-compiled generated flow graph fixture for UI previewing without requiring a running agent.
 */
export const previewGraph: GeneratedFlowGraph = (() => {
  const graph = parseFlowGraph(source);
  return {
    projectId: "0b6f4d3e-3f2a-4b7c-8d1e-9a0c5f2b7d41",
    provider: "codex",
    snapshotHash: "preview",
    generatedAt: new Date("2026-08-29T00:00:00.000Z").toISOString(),
    graph,
    layout: createDefaultGraphLayout(graph),
    layoutVersion: 2,
  };
})();

