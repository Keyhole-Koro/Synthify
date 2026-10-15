import { describe, expect, it } from "vitest";
import { applyScopeExpansion, balanceFlowGraphScopes, buildPortalPreview, createDefaultGraphLayout, generatedFlowGraphSchema, layoutFlowNodes, layoutRootNodes, parseFlowGraph, parseFlowGraphExpansion, projectFlowToScope, scopeBoundaryPorts, validateScopeExpansion } from "./index.js";

const graph = {
  title: "System",
  summary: "A small system",
  nodes: [
    { id: "input", title: "Input", summary: "Accept input", kind: "process", parentId: null, evidence: ["src/input.ts"] },
    { id: "output", title: "Output", summary: "Return output", kind: "process", parentId: null, evidence: ["src/output.ts"] },
  ],
  edges: [{ source: "input", target: "output", label: "passes data" }],
} as const;

describe("flowGraphSchema", () => {
  it("accepts a referenced graph", () => {
    expect(parseFlowGraph(graph).nodes).toHaveLength(2);
  });

  it("rejects edges to unknown nodes", () => {
    expect(() => parseFlowGraph({ ...graph, edges: [{ source: "input", target: "missing", label: "bad" }] })).toThrow();
  });
});

describe("layoutRootNodes", () => {
  it("places visible nodes deterministically", () => {
    expect(layoutRootNodes(parseFlowGraph(graph)).map(({ id, x, y }) => ({ id, x, y }))).toEqual([
      { id: "input", x: 15, y: 50 },
      { id: "output", x: 85, y: 50 },
    ]);
  });

  it("lays out an arbitrary nested scope", () => {
    expect(layoutFlowNodes(parseFlowGraph(graph).nodes).map((node) => node.id)).toEqual(["input", "output"]);
  });

  it("uses edge direction instead of input order", () => {
    const parsed = parseFlowGraph(graph);
    const positions = layoutFlowNodes([...parsed.nodes].reverse(), parsed.edges);
    expect(positions.find((node) => node.id === "input")!.x).toBeLessThan(positions.find((node) => node.id === "output")!.x);
  });
});

describe("scope projection", () => {
  const nested = parseFlowGraph({
    ...graph,
    nodes: [
      ...graph.nodes,
      { id: "parse", title: "Parse", summary: "Parse input", kind: "process", parentId: "input", evidence: [] },
      { id: "validate", title: "Validate", summary: "Validate input", kind: "decision", parentId: "input", evidence: [] },
    ],
    edges: [
      { source: "output", target: "parse", label: "request" },
      { source: "parse", target: "validate", label: "parsed" },
      { source: "validate", target: "output", label: "result" },
    ],
  });

  it("keeps flow continuity at Room boundaries", () => {
    const projection = projectFlowToScope(nested, "input");
    expect(projection.nodes.map((node) => node.id)).toEqual(["parse", "validate"]);
    expect(projection.edges).toEqual(expect.arrayContaining([
      expect.objectContaining({ source: null, target: "parse", count: 1 }),
      expect.objectContaining({ source: "parse", target: "validate", count: 1 }),
      expect.objectContaining({ source: "validate", target: null, count: 1 }),
    ]));
  });

  it("bundles descendant edges at their visible Portal", () => {
    const projection = projectFlowToScope(nested, null);
    expect(projection.edges).toEqual([
      expect.objectContaining({ source: "output", target: "input", count: 1 }),
      expect.objectContaining({ source: "input", target: "output", count: 1 }),
    ]);
  });
});

describe("Room density", () => {
  it("keeps unlimited total nodes while folding each Room to seven Portals", () => {
    const many = parseFlowGraph({
      ...graph,
      nodes: Array.from({ length: 18 }, (_, index) => ({ id: `node-${index}`, title: `Node ${index}`, summary: "Step", kind: "process" as const, parentId: null, evidence: [] })),
      edges: [],
    });
    const balanced = balanceFlowGraphScopes(many);
    expect(balanced.nodes.length).toBeGreaterThan(18);
    const parentIds = new Set<string | null>([null, ...balanced.nodes.map((node) => node.parentId)]);
    for (const parentId of parentIds) {
      expect(balanced.nodes.filter((node) => node.parentId === parentId).length).toBeLessThanOrEqual(7);
    }
  });
});

describe("generatedFlowGraphSchema", () => {
  it("validates persisted manual layout coordinates", () => {
    expect(generatedFlowGraphSchema.parse({
      projectId: "55f8b65b-0919-4f86-81a5-1fdb137bb69f",
      provider: "codex",
      snapshotHash: "snapshot",
      generatedAt: "2026-08-29T00:00:00.000Z",
      graph,
      layout: { input: { x: 20, y: 30 } },
    }).layout.input.x).toBe(20);
  });
});

describe("scope expansion", () => {
  it("merges append-only patches without a global node limit", () => {
    const current = parseFlowGraph(graph);
    const nodes = Array.from({ length: 20 }, (_, index) => ({ id: `detail-${index}`, title: `Detail ${index}`, summary: "Nested detail", kind: "process" as const, parentId: "input", evidence: [] }));
    const expansion = parseFlowGraphExpansion({ nodes, edges: [{ source: "detail-0", target: "output", label: "boundary" }] });
    expect(applyScopeExpansion(current, expansion, "input").nodes).toHaveLength(22);
  });

  it("rejects a patch edge unrelated to every generated node", () => {
    const current = parseFlowGraph(graph);
    const expansion = parseFlowGraphExpansion({
      nodes: [{ id: "detail", title: "Detail", summary: "Nested detail", kind: "process", parentId: "input", evidence: [] }],
      edges: [{ source: "output", target: "input", label: "unrelated" }],
    });
    expect(() => applyScopeExpansion(current, expansion, "input")).toThrow("unrelated edge");
  });

  it("preserves existing nodes and only adds descendants of the requested Room", () => {
    const current = parseFlowGraph(graph);
    const expanded = parseFlowGraph({ ...graph, nodes: [...graph.nodes, { id: "detail", title: "Detail", summary: "Nested detail", kind: "process", parentId: "input", evidence: [] }] });
    expect(validateScopeExpansion(current, expanded, "input").nodes).toHaveLength(3);
    expect(createDefaultGraphLayout(expanded)).toHaveProperty("detail");
  });

  it("rejects an expansion that rewrites an existing node", () => {
    const current = parseFlowGraph(graph);
    const expanded = parseFlowGraph({ ...graph, nodes: [{ ...graph.nodes[0], title: "Rewritten" }, graph.nodes[1], { id: "detail", title: "Detail", summary: "Nested detail", kind: "process", parentId: "input", evidence: [] }] });
    expect(() => validateScopeExpansion(current, expanded, "input")).toThrow("changed existing node");
  });

  it("allows a generated child to connect to an existing boundary node", () => {
    const current = parseFlowGraph(graph);
    const expanded = parseFlowGraph({
      ...graph,
      nodes: [...graph.nodes, { id: "detail", title: "Detail", summary: "Nested detail", kind: "process", parentId: "input", evidence: [] }],
      edges: [...graph.edges, { source: "input", target: "detail", label: "crosses scope" }],
    });
    expect(validateScopeExpansion(current, expanded, "input").edges).toHaveLength(2);
  });

  it("rejects an unrelated new edge between existing nodes", () => {
    const current = parseFlowGraph(graph);
    const expanded = parseFlowGraph({
      ...graph,
      nodes: [...graph.nodes, { id: "detail", title: "Detail", summary: "Nested detail", kind: "process", parentId: "input", evidence: [] }],
      edges: [...graph.edges, { source: "output", target: "input", label: "unrelated" }],
    });
    expect(() => validateScopeExpansion(current, expanded, "input")).toThrow("unrelated edge");
  });
});

describe("scope boundary ports", () => {
  const nested = parseFlowGraph({
    ...graph,
    nodes: [
      ...graph.nodes,
      { id: "parse", title: "Parse", summary: "Parse input", kind: "process", parentId: "input", evidence: [] },
      { id: "validate", title: "Validate", summary: "Validate input", kind: "decision", parentId: "input", evidence: [] },
    ],
    edges: [
      { source: "output", target: "parse", label: "request" },
      { source: "parse", target: "validate", label: "parsed" },
      { source: "validate", target: "output", label: "result" },
    ],
  });

  it("names the node on the other side of the Room wall", () => {
    expect(scopeBoundaryPorts(nested, "input")).toEqual([
      { side: "input", nodeId: "parse", count: 1, endpoints: [{ id: "output", title: "Output", label: "request" }] },
      { side: "output", nodeId: "validate", count: 1, endpoints: [{ id: "output", title: "Output", label: "result" }] },
    ]);
  });

  it("keeps an edge that ends at the Room node itself", () => {
    const deep = parseFlowGraph({
      ...graph,
      nodes: [
        ...graph.nodes,
        { id: "stage", title: "Stage", summary: "A nested stage", kind: "room", parentId: "input", evidence: [] },
        { id: "first", title: "First", summary: "Opens the stage", kind: "process", parentId: "stage", evidence: [] },
        { id: "last", title: "Last", summary: "Closes the stage", kind: "process", parentId: "stage", evidence: [] },
      ],
      edges: [
        { source: "output", target: "stage", label: "request" },
        { source: "first", target: "last", label: "step" },
        { source: "stage", target: "output", label: "result" },
      ],
    });
    expect(scopeBoundaryPorts(deep, "stage")).toEqual([
      { side: "input", nodeId: "first", count: 1, endpoints: [{ id: "output", title: "Output", label: "request" }] },
      { side: "output", nodeId: "last", count: 1, endpoints: [{ id: "output", title: "Output", label: "result" }] },
    ]);
  });

  it("has no boundary at the root scope", () => {
    expect(scopeBoundaryPorts(nested, null)).toEqual([]);
  });
});

describe("portal preview", () => {
  const withChildren = parseFlowGraph({
    ...graph,
    nodes: [
      ...graph.nodes,
      ...Array.from({ length: 6 }, (_unused, index) => ({
        id: `step-${index}`,
        title: `Step ${index}`,
        summary: "A step",
        kind: "process" as const,
        parentId: "input",
        evidence: [],
      })),
    ],
    edges: [
      ...graph.edges,
      ...Array.from({ length: 5 }, (_unused, index) => ({ source: `step-${index}`, target: `step-${index + 1}`, label: "" })),
      { source: "step-5", target: "output", label: "done" },
    ],
  });

  it("shows a bounded miniature of the flow inside a Portal", () => {
    const preview = buildPortalPreview(withChildren, "input");
    expect(preview.childCount).toBe(6);
    expect(preview.descendantCount).toBe(6);
    expect(preview.nodes).toHaveLength(5);
    expect(preview.hiddenCount).toBe(1);
    expect(preview.nodes.map((node) => node.id)).toEqual(["step-0", "step-1", "step-2", "step-3", "step-4"]);
    expect(preview.edges).toHaveLength(4);
    expect(preview.nodes.every((node) => node.x >= 0 && node.x <= 100 && node.y >= 0 && node.y <= 100)).toBe(true);
  });

  it("marks where the inner flow crosses the Portal wall", () => {
    const preview = buildPortalPreview(withChildren, "input");
    expect(preview.nodes.find((node) => node.id === "step-0")!.isExit).toBe(false);
    expect(buildPortalPreview(withChildren, "output")).toEqual({ nodes: [], edges: [], childCount: 0, descendantCount: 0, hiddenCount: 0 });
  });
});
