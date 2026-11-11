import { useEffect, useMemo, useRef, useState, type CSSProperties, type PointerEvent as ReactPointerEvent } from "react";
import type { AgentEvent, ProviderInstallation } from "@insightify/agent-runtime";
import type { ExecutableAgentProvider, GraphGenerationEvent, ProjectSummary, StartRunResult } from "@insightify/desktop-bridge";
import {
  buildPortalPreview,
  FLOWFOLD_ROOM_MAX_NODES,
  layoutFlowNodes,
  projectFlowToScope,
  scopeBoundaryPorts,
  type FlowEdge,
  type FlowNode,
  type FlowNodeKind,
  type GeneratedFlowGraph,
  type PortalPreview,
  type ProjectedFlowEdge,
  type ScopeBoundaryPort,
} from "@insightify/graph-domain";
import { PORTAL_CARD_WIDTH, semanticLevelForZoom, stageMetrics, type SemanticLevel } from "./semantic-zoom.js";

/** Approval event payload requesting user decision. */
type ApprovalRequestedEvent = Extract<AgentEvent, { type: "approval.requested" }>;
/** Approval event payload indicating resolution of an approval request. */
type ApprovalResolvedEvent = Extract<AgentEvent, { type: "approval.resolved" }>;
/** Draft state for creating or editing a flow node. */
type NodeDraft = { nodeId: string; title: string; summary: string; kind: FlowNodeKind; evidence: string };
/** Draft state for creating or modifying a connection edge between nodes. */
type EdgeDraft = { index: number | null; source: string; target: string; label: string };
/** Pointer drag state for repositioning nodes on the canvas. */
type DragState = { nodeId: string; pointerId: number; moved: boolean };
/**
 * Entering a Portal is a descent into it, not a page change: the current Room
 * scales into the Portal frame, then the child Room grows out of the same spot.
 */
type DiveState = { phase: "exit" | "enter"; scale: number; x: number; y: number };
/** Projected flow edge guaranteed to connect two defined node endpoints in the active room. */
type RoomEdge = ProjectedFlowEdge & { source: string; target: string };

const emptyNodes: FlowNode[] = [];
const DIVE_MS = 200;
const DIVE_SCALE_IN = 2.6;
const DIVE_SCALE_OUT = 0.42;
const levelLabels: Record<SemanticLevel, string> = { structure: "structure", flow: "flow", implementation: "implementation" };

const providerMeta: Record<ExecutableAgentProvider, { label: string; shortLabel: string; policy: string }> = {
  codex: { label: "Codex", shortLabel: "CX", policy: "workspace-write · interactive approvals" },
  "antigravity-cli": { label: "Antigravity", shortLabel: "AG", policy: "sandbox · configured permissions" },
};
const providerKinds: ExecutableAgentProvider[] = ["antigravity-cli", "codex"];
const nodeKinds: FlowNodeKind[] = ["room", "process", "decision", "data", "external"];

/**
 * Root interactive component for the Insightify desktop workspace.
 * Coordinates project selection, semantic zoom navigation, multi-scale flow folding,
 * and background agent runtime orchestration.
 *
 * @returns The desktop workspace shell with project navigation, graph canvas, generation
 *   controls, approval prompts, and runtime diagnostics.
 */
export function App() {
  const [projects, setProjects] = useState<ProjectSummary[]>([]);
  const [project, setProject] = useState<ProjectSummary | null>(null);
  const projectIdRef = useRef<string | null>(null);
  const [graph, setGraphState] = useState<GeneratedFlowGraph | null>(null);
  const graphRef = useRef<GeneratedFlowGraph | null>(null);
  const saveQueue = useRef(Promise.resolve());
  const [graphLoading, setGraphLoading] = useState(false);
  const [generatingGraph, setGeneratingGraph] = useState(false);
  const [expandingScopeId, setExpandingScopeId] = useState<string | null>(null);
  const [providers, setProviders] = useState<ProviderInstallation[]>([]);
  const [providerKind, setProviderKind] = useState<ExecutableAgentProvider>("antigravity-cli");
  const [events, setEvents] = useState<AgentEvent[]>([]);
  const [prompt, setPrompt] = useState("Review this point in the flow and propose the smallest useful implementation step.");
  const [run, setRun] = useState<StartRunResult | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [currentScopeId, setCurrentScopeId] = useState<string | null>(null);
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);
  const [peekNodeId, setPeekNodeId] = useState<string | null>(null);
  const [zoom, setZoom] = useState(1);
  const [lod, setLod] = useState<SemanticLevel>("flow");
  const [dive, setDive] = useState<DiveState | null>(null);
  const [frame, setFrame] = useState({ width: 960, height: 700 });
  const diveTimers = useRef<ReturnType<typeof setTimeout>[]>([]);
  const [nodeDraft, setNodeDraft] = useState<NodeDraft | null>(null);
  const [edgeManagerOpen, setEdgeManagerOpen] = useState(false);
  const [edgeDraft, setEdgeDraft] = useState<EdgeDraft | null>(null);
  const canvasRef = useRef<HTMLElement | null>(null);
  const dragRef = useRef<DragState | null>(null);

  function setGraph(value: GeneratedFlowGraph | null) {
    graphRef.current = value;
    setGraphState(value);
  }

  useEffect(() => {
    void Promise.all([window.insightify.listProjects(), window.insightify.probeProviders()])
      .then(([knownProjects, installations]) => {
        setProjects(knownProjects);
        setProviders(installations);
        if (!installations.some((item) => item.provider === "antigravity-cli" && item.installed)) {
          if (installations.some((item) => item.provider === "codex" && item.installed)) setProviderKind("codex");
        }
        const initial = knownProjects[0] ?? null;
        setProject(initial);
        projectIdRef.current = initial?.id ?? null;
        if (initial) void loadGraph(initial.id, () => projectIdRef.current, setGraph, setGraphLoading, setError);
      })
      .catch((reason) => setError(toMessage(reason)));

    const offAgent = window.insightify.onAgentEvent((event) => {
      setEvents((current) => [...current.slice(-499), event]);
      if (event.type === "run.completed" || event.type === "run.failed") { setBusy(false); setRun(null); }
    });
    const offGraph = window.insightify.onGraphGeneration((event) => handleGraphEvent(
      event, projectIdRef.current, setGraph, setGeneratingGraph, setExpandingScopeId, setCurrentScopeId, setBusy, setRun, setEvents, setError,
    ));
    return () => { offAgent(); offGraph(); };
  }, []);

  useEffect(() => () => clearDive(diveTimers), []);
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas || typeof ResizeObserver !== "function") return;
    const observer = new ResizeObserver(([entry]) => setFrame({ width: entry.contentRect.width, height: entry.contentRect.height }));
    observer.observe(canvas);
    return () => observer.disconnect();
  }, [project, graph]);

  useEffect(() => {
    if (currentScopeId && graph && !graph.graph.nodes.some((node) => node.id === currentScopeId)) {
      setCurrentScopeId(null); setSelectedNodeId(null); setPeekNodeId(null);
    }
  }, [graph, currentScopeId]);

  const provider = providers.find((item) => item.provider === providerKind) ?? null;
  const meta = providerMeta[providerKind];
  const projection = useMemo(() => (graph ? projectFlowToScope(graph.graph, currentScopeId) : null), [graph, currentScopeId]);
  const visibleNodes = projection?.nodes ?? emptyNodes;
  // Edges between deeper descendants are bundled onto the Portals that own
  // them, so a Room never shows a step without showing what feeds it.
  const roomEdges = useMemo(() => (projection?.edges ?? []).filter(isRoomEdge), [projection]);
  const boundaryPorts = useMemo(() => (graph ? scopeBoundaryPorts(graph.graph, currentScopeId) : []), [graph, currentScopeId]);
  // The stage is measured from the computed flow, not from dragged positions,
  // so moving one Portal never resizes the Room under the others.
  const flowLayout = useMemo(() => layoutFlowNodes(visibleNodes, roomEdges.map(toFlowEdge)), [visibleNodes, roomEdges]);
  const stage = useMemo(() => stageMetrics(flowLayout, frame), [flowLayout, frame]);
  const stageZoom = zoom * stage.scale;
  const positionedNodes = useMemo(() => flowLayout.map((node) => ({ ...node, ...(graph?.layout[node.id] ?? {}) })), [flowLayout, graph]);
  const visibleIds = useMemo(() => new Set(visibleNodes.map((node) => node.id)), [visibleNodes]);
  const editableEdges = useMemo(() => graph?.graph.edges.map((edge, index) => ({ edge, index })).filter(({ edge }) => visibleIds.has(edge.source) && visibleIds.has(edge.target)) ?? [], [graph, visibleIds]);
  const previews = useMemo(() => new Map(graph ? visibleNodes.map((node) => [node.id, buildPortalPreview(graph.graph, node.id)] as const) : []), [visibleNodes, graph]);
  const positions = useMemo(() => new Map(positionedNodes.map((node) => [node.id, node])), [positionedNodes]);
  // Chips and labels are drawn on the frame at full size, so they stay legible
  // however far the Room had to be scaled down to fit.
  const projected = useMemo(() => frameProjection(stage.width, stage.height, stageZoom, frame), [stage, stageZoom, frame]);
  const portRail = useMemo(() => layoutBoundaryRail(boundaryPorts, positionedNodes, projected), [boundaryPorts, positionedNodes, projected]);
  const selectedNode = graph?.graph.nodes.find((node) => node.id === selectedNodeId) ?? null;
  const peekNode = graph?.graph.nodes.find((node) => node.id === peekNodeId) ?? null;
  const scopePath = useMemo(() => buildScopePath(graph, currentScopeId), [graph, currentScopeId]);
  const scopeNode = currentScopeId ? graph?.graph.nodes.find((node) => node.id === currentScopeId) ?? null : null;
  useEffect(() => setLod((current) => semanticLevelForZoom(current, stageZoom)), [stageZoom]);
  const transcript = useMemo(() => events.filter((event) => event.type === "assistant.delta").map((event) => event.text).join(""), [events]);
  const approvals = useMemo(() => {
    const resolved = new Set(events.filter(isApprovalResolved).map((event) => event.requestId));
    return events.filter(isApprovalRequested).filter((event) => !resolved.has(event.requestId));
  }, [events]);

  function resetView() {
    clearDive(diveTimers); setDive(null);
    setCurrentScopeId(null); setSelectedNodeId(null); setPeekNodeId(null); setEvents([]); setRun(null); setZoom(1);
  }

  function selectProject(selected: ProjectSummary) {
    projectIdRef.current = selected.id;
    setProject(selected); setGraph(null); resetView(); setError(null);
    void loadGraph(selected.id, () => projectIdRef.current, setGraph, setGraphLoading, setError);
  }

  async function pickProject() {
    setError(null);
    try {
      const selected = await window.insightify.pickProject();
      if (!selected) return;
      setProjects((current) => [selected, ...current.filter((item) => item.id !== selected.id)]);
      selectProject(selected);
    } catch (reason) { setError(toMessage(reason)); }
  }

  function persistGraph(value: GeneratedFlowGraph) {
    setGraph(value);
    saveQueue.current = saveQueue.current
      .then(async () => { await window.insightify.saveProjectGraph(value); })
      .catch((reason) => setError(`Could not save graph: ${toMessage(reason)}`));
  }

  function locallyUpdateGraph(update: (current: GeneratedFlowGraph) => GeneratedFlowGraph) {
    const current = graphRef.current;
    if (!current) return null;
    const next = update(current);
    setGraph(next);
    return next;
  }

  function mutateGraph(update: (current: GeneratedFlowGraph) => GeneratedFlowGraph) {
    const current = graphRef.current;
    if (current) persistGraph(update(current));
  }

  async function generateGraph(scopeNodeId?: string) {
    if (!project || !provider?.installed) return;
    setBusy(true); setGeneratingGraph(!scopeNodeId); setExpandingScopeId(scopeNodeId ?? null); setError(null); setEvents([]);
    try { setRun(await window.insightify.generateFlowGraph({ provider: providerKind, projectId: project.id, ...(scopeNodeId ? { scopeNodeId } : {}) })); }
    catch (reason) { setBusy(false); setGeneratingGraph(false); setExpandingScopeId(null); setError(toMessage(reason)); }
  }

  async function startRun() {
    if (!project || !prompt.trim()) return;
    setBusy(true); setError(null); setEvents([]);
    const anchor = selectedNode ?? scopeNode;
    const anchoredPrompt = [
      "INSIGHTIFY_FLOWFOLD_ANCHOR",
      `Project: ${project.displayName}`,
      `Graph: ${graph?.graph.title ?? "not generated"}`,
      `Room path: Root${scopePath.map((node) => ` / ${node.title}`).join("")}`,
      anchor ? `Selected node: ${anchor.title}\nNode kind: ${anchor.kind}\nNode summary: ${anchor.summary}\nEvidence: ${anchor.evidence.join(", ") || "none"}` : "Selected node: root scope",
      "Treat this anchor as the working context. Do not modify unrelated areas.",
      "",
      prompt,
    ].join("\n");
    try { setRun(await window.insightify.startAgentRun({ provider: providerKind, projectId: project.id, prompt: anchoredPrompt })); }
    catch (reason) { setBusy(false); setError(toMessage(reason)); }
  }

  async function cancelRun() {
    if (!project || !run) return;
    try { await window.insightify.cancelAgentRun({ provider: providerKind, projectId: project.id, threadId: run.threadId, runId: run.runId }); }
    catch (reason) { setError(toMessage(reason)); }
  }

  function enterRoom(node: FlowNode) {
    if (busy) return;
    const origin = positions.get(node.id);
    diveTo(DIVE_SCALE_IN, origin?.x ?? 50, origin?.y ?? 50, () => {
      setCurrentScopeId(node.id); setSelectedNodeId(null); setPeekNodeId(null); setEvents([]); setZoom(1);
      const hasChildren = graphRef.current?.graph.nodes.some((candidate) => candidate.parentId === node.id) ?? false;
      if (!hasChildren) void generateGraph(node.id);
    });
  }

  function navigateToScope(scopeId: string | null) {
    if (busy) return;
    // Leaving reverses the descent: the Room shrinks back into the Portal that
    // owns it, so the parent never appears as an unrelated screen. Jumping
    // several levels lands on the Portal that owns the target Room, not on the
    // deep Node we happened to be standing in.
    const ownerId = ancestorWithin(graph, currentScopeId, scopeId);
    const owner = ownerId ? graph?.layout[ownerId] : undefined;
    diveTo(DIVE_SCALE_OUT, owner?.x ?? 50, owner?.y ?? 50, () => {
      setCurrentScopeId(scopeId); setSelectedNodeId(ownerId); setPeekNodeId(null); setEvents([]); setZoom(1);
    });
  }

  function diveTo(scale: number, x: number, y: number, commit: () => void) {
    clearDive(diveTimers);
    if (prefersReducedMotion()) { setDive(null); commit(); return; }
    setDive({ phase: "exit", scale, x, y });
    diveTimers.current.push(setTimeout(() => {
      commit();
      setDive({ phase: "enter", scale, x, y });
      diveTimers.current.push(setTimeout(() => setDive(null), DIVE_MS + 40));
    }, DIVE_MS));
  }

  function openEditNode(node: FlowNode) {
    setSelectedNodeId(node.id);
    setNodeDraft({ nodeId: node.id, title: node.title, summary: node.summary, kind: node.kind, evidence: node.evidence.join("\n") });
  }

  function saveNodeDraft() {
    if (!nodeDraft || !nodeDraft.title.trim() || !nodeDraft.summary.trim() || !graphRef.current) return;
    mutateGraph((current) => ({ ...current, graph: { ...current.graph, nodes: current.graph.nodes.map((node) => node.id === nodeDraft.nodeId ? { ...node, title: nodeDraft.title.trim(), summary: nodeDraft.summary.trim(), kind: nodeDraft.kind, evidence: parseEvidence(nodeDraft.evidence) } : node) } }));
    setNodeDraft(null);
  }

  function deleteNode(nodeId: string) {
    const current = graphRef.current;
    const node = current?.graph.nodes.find((item) => item.id === nodeId);
    if (!current || !node || !window.confirm(`Delete “${node.title}” and all nested nodes?`)) return;
    const removed = descendantIds(current.graph.nodes, nodeId);
    mutateGraph((value) => ({
      ...value,
      graph: {
        ...value.graph,
        nodes: value.graph.nodes.filter((item) => !removed.has(item.id)),
        edges: value.graph.edges.filter((edge) => !removed.has(edge.source) && !removed.has(edge.target)),
      },
      layout: Object.fromEntries(Object.entries(value.layout).filter(([id]) => !removed.has(id))),
    }));
    setNodeDraft(null); setSelectedNodeId(null); setPeekNodeId(null);
  }

  function startNewEdge() {
    if (visibleNodes.length < 2) { setError("Create at least two nodes in this Room before connecting them."); return; }
    setEdgeDraft({ index: null, source: visibleNodes[0].id, target: visibleNodes[1].id, label: "" });
  }

  function saveEdgeDraft() {
    if (!edgeDraft || edgeDraft.source === edgeDraft.target) { setError("An edge must connect two different nodes."); return; }
    const edge: FlowEdge = { source: edgeDraft.source, target: edgeDraft.target, label: edgeDraft.label.trim() };
    mutateGraph((current) => ({ ...current, graph: { ...current.graph, edges: edgeDraft.index === null ? [...current.graph.edges, edge] : current.graph.edges.map((item, index) => index === edgeDraft.index ? edge : item) } }));
    setEdgeDraft(null);
  }

  function deleteEdge(index: number) {
    mutateGraph((current) => ({ ...current, graph: { ...current.graph, edges: current.graph.edges.filter((_edge, edgeIndex) => edgeIndex !== index) } }));
    setEdgeDraft(null);
  }

  function onNodePointerDown(event: ReactPointerEvent<HTMLElement>, nodeId: string) {
    if (event.button !== 0 || busy || (event.target as HTMLElement).closest("button")) return;
    event.currentTarget.setPointerCapture(event.pointerId);
    dragRef.current = { nodeId, pointerId: event.pointerId, moved: false };
    setSelectedNodeId(nodeId); setPeekNodeId(null);
  }

  function onNodePointerMove(event: ReactPointerEvent<HTMLElement>) {
    const drag = dragRef.current;
    const canvas = canvasRef.current;
    if (!drag || drag.pointerId !== event.pointerId || !canvas) return;
    const rect = canvas.getBoundingClientRect();
    const x = clamp(50 + ((event.clientX - rect.left - rect.width / 2) / stageZoom / stage.width) * 100, 8, 92);
    const y = clamp(50 + ((event.clientY - rect.top - rect.height / 2) / stageZoom / stage.height) * 100, 15, 87);
    drag.moved = true;
    locallyUpdateGraph((current) => ({ ...current, layout: { ...current.layout, [drag.nodeId]: { x, y } } }));
  }

  function onNodePointerUp(event: ReactPointerEvent<HTMLElement>) {
    const drag = dragRef.current;
    if (!drag || drag.pointerId !== event.pointerId) return;
    dragRef.current = null;
    if (drag.moved && graphRef.current) persistGraph(graphRef.current);
  }

  return (
    <div className="app-shell">
      <aside className="project-rail">
        <div className="brand-mark" aria-label="Insightify">I</div>
        <button className="rail-action" onClick={pickProject} title="Open project">＋</button>
        <div className="project-list">{projects.map((item) => <button className={item.id === project?.id ? "project-chip selected" : "project-chip"} key={item.id} disabled={busy} onClick={() => selectProject(item)} title={item.displayName}>{item.displayName.slice(0, 2).toUpperCase()}</button>)}</div>
      </aside>

      <main className="workspace">
        <header className="topbar">
          <div>
            <nav className="breadcrumbs" aria-label="Room breadcrumb">
              <span>Insightify</span><b>/</b><span>{project?.displayName ?? "No project"}</span><b>/</b>
              {graph && <button onClick={() => navigateToScope(null)}>{graph.graph.title}</button>}
              {scopePath.map((node) => <span className="breadcrumb-segment" key={node.id}><b>/</b><button onClick={() => navigateToScope(node.id)}>{node.title}</button></span>)}
            </nav>
            <div className="room-meta">{scopeNode ? "Nested Room" : "Root scope"} · <span className={visibleNodes.length > FLOWFOLD_ROOM_MAX_NODES ? "over-dense" : undefined}>{visibleNodes.length}/{FLOWFOLD_ROOM_MAX_NODES} portals</span> · {boundaryPorts.length} boundary ports · semantic {levelLabels[lod]}</div>
          </div>
          <div className="topbar-actions">
            {project && graph && <button className="quiet-button" disabled={busy || !provider?.installed} onClick={() => void generateGraph()}>Regenerate</button>}
            <ProviderSwitcher providers={providers} selected={providerKind} busy={busy} onSelect={(kind) => { setProviderKind(kind); setEvents([]); setRun(null); setError(null); }} />
            <button className="quiet-button">Share</button>
          </div>
        </header>

        <section className="canvas-frame" ref={canvasRef}>
          {!project && <EmptyProject onPick={pickProject} />}
          {project && !graph && <GraphEmpty loading={graphLoading} generating={generatingGraph} provider={provider} meta={meta} run={run} onGenerate={generateGraph} onCancel={cancelRun} />}
          {project && graph && (
            <>
              <div className="canvas-toolbar">
                {currentScopeId && <button onClick={() => navigateToScope(scopeNode?.parentId ?? null)}>← Back</button>}
                {currentScopeId && <button disabled={busy} onClick={() => void generateGraph(currentScopeId)}>✦ Expand with {meta.label}</button>}
                <button onClick={() => { setEdgeManagerOpen(true); setEdgeDraft(null); }}>↗ Edges <span>{editableEdges.length}</span></button>
              </div>
              <div className="scope-label"><span>ROOM</span> {scopeNode?.title ?? graph.graph.title}</div>
              {visibleNodes.length === 0 && <div className="empty-scope"><strong>{expandingScopeId === currentScopeId ? `${meta.label} is decomposing this Room…` : "This Room has not been decomposed yet."}</strong><p>{expandingScopeId === currentScopeId ? "Existing Graph entities are locked while new child Nodes are validated." : "Room expansion normally starts automatically when you enter."}</p>{expandingScopeId === currentScopeId ? <button className="stop-button" onClick={cancelRun}>Stop expansion</button> : currentScopeId && <button className="primary-button" disabled={busy || !provider?.installed} onClick={() => void generateGraph(currentScopeId)}>Retry with {meta.label}</button>}</div>}
              <div className="room-stack" aria-hidden="true">{scopePath.slice(-3).map((node, index) => <i key={node.id} style={{ "--fold": index + 1 } as CSSProperties} />)}</div>
              <div
                className={`graph-stage lod-${lod}${dive ? ` dive-${dive.phase}` : ""}`}
                style={{ width: stage.width, height: stage.height, "--stage-zoom": stageZoom, "--dive-scale": dive?.scale ?? 1, "--dive-x": `${dive?.x ?? 50}%`, "--dive-y": `${dive?.y ?? 50}%` } as CSSProperties}
              >
                <svg className="edge-layer" viewBox="0 0 1000 620" preserveAspectRatio="none" aria-hidden="true">
                  <defs>
                    <marker id="flow-arrow" markerWidth="8" markerHeight="8" refX="7" refY="3.4" orient="auto"><path d="M 0 0 L 8 3.4 L 0 6.8 z" /></marker>
                  </defs>
                  {roomEdges.map((edge) => {
                    const source = positions.get(edge.source); const target = positions.get(edge.target);
                    if (!source || !target) return null;
                    const sx = source.x * 10; const sy = source.y * 6.2; const tx = target.x * 10; const ty = target.y * 6.2;
                    const label = edge.count > 1 ? `${edge.labels[0] ?? "flow"} ×${edge.count}` : edge.labels[0] ?? "";
                    return (
                      <g className={edge.count > 1 ? "flow-edge bundled" : "flow-edge"} key={`${edge.source}-${edge.target}`}>
                        <path d={`M ${sx} ${sy} C ${(sx + tx) / 2} ${sy}, ${(sx + tx) / 2} ${ty}, ${tx} ${ty}`} markerEnd="url(#flow-arrow)" />
                      </g>
                    );
                  })}
                </svg>
                {positionedNodes.map((node) => (
                  <PortalCard
                    key={node.id}
                    node={node}
                    preview={previews.get(node.id) ?? emptyPreview}
                    lod={lod}
                    selected={selectedNodeId === node.id}
                    connections={connectionSides(node.id, roomEdges, boundaryPorts)}
                    onSelect={() => { if (!dragRef.current) { setSelectedNodeId(node.id); setEvents([]); } }}
                    onPeek={() => setPeekNodeId(node.id)}
                    onEnter={() => enterRoom(node)}
                    onEdit={() => openEditNode(node)}
                    onPointerDown={(event) => onNodePointerDown(event, node.id)}
                    onPointerMove={onNodePointerMove}
                    onPointerUp={onNodePointerUp}
                  />
                ))}
              </div>
              {/* Labels ride on the frame, not on the scaled stage: a Portal gap is
                  narrower than the words, and stretched SVG text is unreadable. */}
              {!dive && lod !== "structure" && roomEdges.map((edge) => {
                const source = positions.get(edge.source); const target = positions.get(edge.target);
                const label = edge.count > 1 ? `${edge.labels[0] ?? "flow"} ×${edge.count}` : edge.labels[0] ?? "";
                if (!source || !target || !label) return null;
                return (
                  <span
                    className="edge-label"
                    key={`label-${edge.source}-${edge.target}`}
                    style={{
                      left: `${projected.x((source.x + target.x) / 2)}%`,
                      top: `${projected.y((source.y + target.y) / 2) - projected.cardHalfHeight - 2.2}%`,
                    }}
                  >
                    {label}
                  </span>
                );
              })}
              {!dive && portRail.length > 0 && (
                <svg className="boundary-layer" viewBox="0 0 100 100" preserveAspectRatio="none" aria-hidden="true">
                  <defs><marker id="boundary-arrow" markerWidth="8" markerHeight="8" refX="7" refY="3.4" orient="auto"><path d="M 0 0 L 8 3.4 L 0 6.8 z" /></marker></defs>
                  {portRail.map((placed) => (
                    // Routed through the clear margin above the flow, so a link
                    // to a distant Portal never crosses the cards in between.
                    <path
                      className="boundary-link"
                      d={placed.port.side === "input"
                        ? `M ${placed.fromX} ${placed.fromY} L ${placed.toX} ${placed.fromY} L ${placed.toX} ${placed.toY}`
                        : `M ${placed.toX} ${placed.toY} L ${placed.toX} ${placed.fromY} L ${placed.fromX} ${placed.fromY}`}
                      key={`link-${portKey(placed.port)}`}
                      markerEnd="url(#boundary-arrow)"
                    />
                  ))}
                </svg>
              )}
              {!dive && portRail.map((placed) => <BoundaryPortChip key={portKey(placed.port)} port={placed.port} lod={lod} left={placed.chipX} top={placed.chipY} />)}
              <div className="zoom-indicator"><button aria-label="Show less detail" onClick={() => setZoom((value) => clamp(value - .15, .55, 1.45))}>−</button><span>{Math.round(stageZoom * 100)}% · {levelLabels[lod]}</span><button aria-label="Show more detail" onClick={() => setZoom((value) => clamp(value + .15, .55, 1.45))}>＋</button></div>
              {peekNode && <PeekPanel node={peekNode} graph={graph} onClose={() => setPeekNodeId(null)} onEnter={() => enterRoom(peekNode)} onEdit={() => openEditNode(peekNode)} />}
              {edgeManagerOpen && <EdgeManager nodes={visibleNodes} edges={editableEdges} draft={edgeDraft} onDraft={setEdgeDraft} onNew={startNewEdge} onSave={saveEdgeDraft} onDelete={deleteEdge} onClose={() => { setEdgeManagerOpen(false); setEdgeDraft(null); }} />}
            </>
          )}
        </section>
      </main>

      <ThreadPanel project={project} graph={graph} anchor={selectedNode ?? scopeNode} events={events} approvals={approvals} transcript={transcript} generating={generatingGraph || expandingScopeId !== null} provider={provider} providerKind={providerKind} meta={meta} prompt={prompt} busy={busy} run={run} error={error} onPrompt={setPrompt} onRun={startRun} onCancel={cancelRun} onError={setError} />

      {nodeDraft && <NodeEditor draft={nodeDraft} onChange={setNodeDraft} onSave={saveNodeDraft} onDelete={nodeDraft.nodeId ? () => deleteNode(nodeDraft.nodeId!) : null} onClose={() => setNodeDraft(null)} />}
    </div>
  );
}

/**
 * Control bar allowing user to switch between installed agent runtime providers (e.g. Codex or Antigravity).
 */
function ProviderSwitcher({ providers, selected, busy, onSelect }: { providers: ProviderInstallation[]; selected: ExecutableAgentProvider; busy: boolean; onSelect(kind: ExecutableAgentProvider): void }) {
  return <div className="provider-switcher" aria-label="Agent provider">{providerKinds.map((kind) => { const installation = providers.find((item) => item.provider === kind); return <button className={`${selected === kind ? "provider-option selected" : "provider-option"} ${installation?.installed ? "online" : "offline"}`} disabled={busy || !installation?.installed} key={kind} onClick={() => onSelect(kind)} title={installation?.installed ? installation.version ?? providerMeta[kind].label : `${providerMeta[kind].label} CLI not detected`}><b>{providerMeta[kind].shortLabel}</b><span>{providerMeta[kind].label}</span><i /></button>; })}</div>;
}

/**
 * Interactive card component representing a node or portal in the active room.
 * Displays level-of-detail information depending on current semantic zoom.
 */
function PortalCard({ node, preview, lod, selected, connections, onSelect, onPeek, onEnter, onEdit, onPointerDown, onPointerMove, onPointerUp }: {
  node: FlowNode & { x: number; y: number };
  preview: PortalPreview;
  lod: SemanticLevel;
  selected: boolean;
  connections: { input: boolean; output: boolean };
  onSelect(): void;
  onPeek(): void;
  onEnter(): void;
  onEdit(): void;
  onPointerDown(event: ReactPointerEvent<HTMLElement>): void;
  onPointerMove(event: ReactPointerEvent<HTMLElement>): void;
  onPointerUp(event: ReactPointerEvent<HTMLElement>): void;
}) {
  const isPortal = preview.childCount > 0;
  return (
    <article
      aria-label={`${node.title}. ${node.kind}. ${isPortal ? `Portal containing ${preview.descendantCount} nodes` : "No inner flow yet"}`}
      className={`portal-card${selected ? " selected" : ""}${isPortal ? " is-portal" : ""}`}
      role="button"
      style={{ left: `${node.x}%`, top: `${node.y}%` }}
      tabIndex={0}
      onClick={onSelect}
      onDoubleClick={onEnter}
      onKeyDown={(event) => { if (event.key === "Enter") { event.preventDefault(); onEnter(); } }}
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
      onPointerCancel={onPointerUp}
    >
      <span className={connections.input ? "card-port in connected" : "card-port in"} aria-hidden="true" />
      <span className={connections.output ? "card-port out connected" : "card-port out"} aria-hidden="true" />
      <div className="portal-header"><span className="portal-icon">{kindIcon(node.kind)}</span><span className="node-kind">{node.kind}</span><span className="status-dot" /></div>
      <h2>{node.title}</h2>
      {lod !== "structure" && <p className="portal-summary">{node.summary}</p>}
      {isPortal && <PortalFold preview={preview} lod={lod} />}
      {lod === "implementation" && <div className="portal-evidence">{node.evidence[0] ?? "No evidence link"}</div>}
      {lod !== "structure" && (
        <div className="portal-footer">
          <span>{isPortal ? `${preview.childCount} inside · ${preview.descendantCount} deep` : "no inner flow"}</span>
          <div>
            <button onClick={(event) => { event.stopPropagation(); onPeek(); }}>Peek</button>
            <button onClick={(event) => { event.stopPropagation(); onEnter(); }}>Enter</button>
            <button aria-label={`Edit ${node.title}`} onClick={(event) => { event.stopPropagation(); onEdit(); }}>•••</button>
          </div>
        </div>
      )}
    </article>
  );
}

/**
 * The folded sheet: a cached miniature of the flow one level down. It is a
 * summary snapshot, never a live child canvas (interaction spec 9.6, 24.2).
 */
function PortalFold({ preview, lod }: { preview: PortalPreview; lod: SemanticLevel }) {
  if (lod === "structure") return <div className="portal-fold collapsed"><i />{preview.childCount} folded</div>;
  const at = (id: string) => preview.nodes.find((node) => node.id === id);
  return (
    <div className="portal-fold">
      <svg viewBox="0 0 100 60" preserveAspectRatio="none" aria-hidden="true">
        {preview.edges.map((edge) => {
          const source = at(edge.source); const target = at(edge.target);
          return source && target ? <line key={`${edge.source}-${edge.target}`} x1={source.x} y1={source.y * .6} x2={target.x} y2={target.y * .6} /> : null;
        })}
      </svg>
      {preview.nodes.map((node) => <i className={`fold-node${node.isEntry ? " entry" : ""}${node.isExit ? " exit" : ""}`} key={node.id} style={{ left: `${node.x}%`, top: `${node.y}%` }} title={node.title} />)}
      {preview.hiddenCount > 0 && <b>+{preview.hiddenCount}</b>}
      {lod === "implementation" && <ol>{preview.nodes.slice(0, 3).map((node) => <li key={node.id}>{node.title}</li>)}</ol>}
    </div>
  );
}

/**
 * A Room is a window on a larger flow, not a sealed box: edges that cross the
 * wall stay visible as a named port on the boundary.
 */
function BoundaryPortChip({ port, lod, left, top }: { port: ScopeBoundaryPort; lod: SemanticLevel; left: number; top: number }) {
  const names = port.endpoints.map((endpoint) => endpoint.title).join(", ");
  return (
    <div className={`boundary-port ${port.side}`} style={{ left: `${left}%`, top: `${top}%` }} title={`${port.side === "input" ? "From" : "To"} ${names}`}>
      <i />
      {lod !== "structure" && <span>{port.side === "input" ? "◀" : "▶"} {names}</span>}
      {port.count > 1 && <b>{port.count}</b>}
    </div>
  );
}

/**
 * Placeholder view rendered when a project has no generated flow graph yet.
 */
function GraphEmpty({ loading, generating, provider, meta, run, onGenerate, onCancel }: { loading: boolean; generating: boolean; provider: ProviderInstallation | null; meta: { label: string }; run: StartRunResult | null; onGenerate(): void; onCancel(): void }) {
  return <div className="empty-project graph-empty"><div className={generating ? "empty-orbit generating" : "empty-orbit"} /><h1>{loading ? "Loading FlowFold Graph…" : generating ? `${meta.label} is mapping the project…` : "Turn this repository into a FlowFold Graph."}</h1><p>{generating ? "A safe project snapshot is being analyzed. Layout is computed locally after validation." : "The selected AI identifies semantic rooms and flows. Generation only starts when you ask."}</p>{!loading && !generating && <button className="primary-button" disabled={!provider?.installed} onClick={onGenerate}>Generate with {meta.label}</button>}{generating && run && <button className="stop-button" onClick={onCancel}>Stop generation</button>}</div>;
}

/**
 * Splash screen view prompt shown when no workspace repository is currently loaded.
 */
function EmptyProject({ onPick }: { onPick(): void }) {
  return <div className="empty-project"><div className="empty-orbit" /><h1>Open a repository to enter its flow.</h1><p>The path stays on this device. Insightify stores an opaque project id in the UI.</p><button className="primary-button" onClick={onPick}>Open project</button></div>;
}

/**
 * Side panel providing a quick preview of node metadata, child rooms, and evidence references.
 */
function PeekPanel({ node, graph, onClose, onEnter, onEdit }: { node: FlowNode; graph: GeneratedFlowGraph; onClose(): void; onEnter(): void; onEdit(): void }) {
  const children = graph.graph.nodes.filter((item) => item.parentId === node.id);
  const connected = graph.graph.edges.filter((edge) => edge.source === node.id || edge.target === node.id);
  return <aside className="peek-panel"><header><span>PEEK · {node.kind}</span><button onClick={onClose}>×</button></header><h2>{node.title}</h2><p>{node.summary}</p><section><b>Nested nodes</b>{children.length ? children.map((child) => <span key={child.id}>{kindIcon(child.kind)} {child.title}</span>) : <em>None yet</em>}</section><section><b>Evidence</b>{node.evidence.length ? node.evidence.map((item) => <code key={item}>{item}</code>) : <em>No artifact links</em>}</section><section><b>Connections</b><em>{connected.length} edges</em></section><footer><button onClick={onEdit}>Edit node</button><button className="primary-button" onClick={onEnter}>Enter Room</button></footer></aside>;
}

/**
 * Modal dialog for inspecting and modifying node properties (title, summary, kind, evidence).
 */
function NodeEditor({ draft, onChange, onSave, onDelete, onClose }: { draft: NodeDraft; onChange(value: NodeDraft): void; onSave(): void; onDelete: (() => void) | null; onClose(): void }) {
  return <div className="modal-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}><form className="editor-modal" onSubmit={(event) => { event.preventDefault(); onSave(); }}><header><div><span>EDIT NODE</span><h2>{draft.title}</h2></div><button type="button" onClick={onClose}>×</button></header><label>Title<input autoFocus value={draft.title} maxLength={60} onChange={(event) => onChange({ ...draft, title: event.target.value })} /></label><label>Summary<textarea value={draft.summary} maxLength={240} onChange={(event) => onChange({ ...draft, summary: event.target.value })} /></label><label>Kind<select value={draft.kind} onChange={(event) => onChange({ ...draft, kind: event.target.value as FlowNodeKind })}>{nodeKinds.map((kind) => <option key={kind}>{kind}</option>)}</select></label><label>Evidence paths<textarea className="evidence-input" value={draft.evidence} placeholder="src/main.ts&#10;docs/design.md" onChange={(event) => onChange({ ...draft, evidence: event.target.value })} /></label><footer>{onDelete ? <button type="button" className="danger-button" onClick={onDelete}>Delete node</button> : <span />}<div><button type="button" onClick={onClose}>Cancel</button><button className="primary-button" disabled={!draft.title.trim() || !draft.summary.trim()}>Save node</button></div></footer></form></div>;
}

/**
 * Side panel for listing, creating, and modifying connection edges within the current room scope.
 */
function EdgeManager({ nodes, edges, draft, onDraft, onNew, onSave, onDelete, onClose }: { nodes: FlowNode[]; edges: Array<{ edge: FlowEdge; index: number }>; draft: EdgeDraft | null; onDraft(value: EdgeDraft | null): void; onNew(): void; onSave(): void; onDelete(index: number): void; onClose(): void }) {
  return <aside className="edge-manager"><header><div><span>ROOM EDGES</span><h2>Connections</h2></div><button onClick={onClose}>×</button></header><div className="edge-list">{edges.length === 0 && <p>No edges in this Room.</p>}{edges.map(({ edge, index }) => <button className={draft?.index === index ? "selected" : ""} key={index} onClick={() => onDraft({ index, ...edge })}><b>{nodeTitle(nodes, edge.source)} → {nodeTitle(nodes, edge.target)}</b><span>{edge.label || "Unlabelled flow"}</span></button>)}</div>{draft ? <div className="edge-form"><label>From<select value={draft.source} onChange={(event) => onDraft({ ...draft, source: event.target.value })}>{nodes.map((node) => <option value={node.id} key={node.id}>{node.title}</option>)}</select></label><label>To<select value={draft.target} onChange={(event) => onDraft({ ...draft, target: event.target.value })}>{nodes.map((node) => <option value={node.id} key={node.id}>{node.title}</option>)}</select></label><label>Label<input maxLength={60} value={draft.label} onChange={(event) => onDraft({ ...draft, label: event.target.value })} /></label><div>{draft.index !== null && <button className="danger-button" onClick={() => onDelete(draft.index!)}>Delete</button>}<button className="primary-button" onClick={onSave}>Save edge</button></div></div> : <button className="primary-button new-edge" onClick={onNew}>＋ New edge</button>}</aside>;
}

/**
 * Thread panel facilitating anchored conversation with the AI agent runtime, displaying
 * live streams, approval requests, and execution controls.
 */
function ThreadPanel({ project, graph, anchor, events, approvals, transcript, generating, provider, providerKind, meta, prompt, busy, run, error, onPrompt, onRun, onCancel, onError }: { project: ProjectSummary | null; graph: GeneratedFlowGraph | null; anchor: FlowNode | null; events: AgentEvent[]; approvals: ApprovalRequestedEvent[]; transcript: string; generating: boolean; provider: ProviderInstallation | null; providerKind: ExecutableAgentProvider; meta: { label: string; policy: string }; prompt: string; busy: boolean; run: StartRunResult | null; error: string | null; onPrompt(value: string): void; onRun(): void; onCancel(): void; onError(value: string | null): void }) {
  return <aside className="thread-panel"><header className="thread-header"><div><span className="eyebrow">ANCHORED THREAD</span><h2>{anchor?.title ?? graph?.graph.title ?? "System Room"}</h2></div><span className="thread-count">{events.length}</span></header><div className="thread-body"><div className="context-card"><span>{anchor ? `Node · ${anchor.kind}` : "Root context"}</span><strong>{anchor?.title ?? project?.displayName ?? "Select a project"}</strong><p>{anchor?.summary ?? graph?.graph.summary ?? "Generate a semantic graph to establish the root scope."}</p>{anchor?.evidence.map((item) => <code key={item}>{item}</code>)}</div>{generating && <><div className="generation-card"><i /><div><strong>Generating FlowFold Graph</strong><p>{generationPhase(events, transcript)} · read-only · validating locally</p></div></div><GraphGenerationStream transcript={transcript} expansion={anchor !== null} /></>}{!generating && transcript && <div className="assistant-message"><span>{meta.label}</span><p>{transcript}</p></div>}{!generating && !transcript && project && <div className="thread-placeholder">Select any Node to anchor this conversation. Its Room path, summary and evidence are compiled into the run.</div>}{approvals.map((approval) => <div className="approval-card" key={approval.requestId}><span>{approval.approvalKind === "command" ? "Command approval" : "File change approval"}</span><code>{approval.command ?? approval.reason ?? "Review proposed change"}</code><div><button onClick={() => respond(project, approval.requestId, "decline", onError)}>Decline</button><button className="approve" onClick={() => respond(project, approval.requestId, "accept", onError)}>Approve once</button></div></div>)}{error && <div className="error-card">{error}</div>}{!provider?.installed && <div className="provider-help"><strong>{meta.label} CLI is not available</strong><code>{providerKind === "codex" ? "codex --version" : "agy --version"}</code></div>}</div><footer className="composer"><textarea value={prompt} onChange={(event) => onPrompt(event.target.value)} placeholder="Ask from this point in the flow…" disabled={!project || busy} /><div className="composer-footer"><span>{meta.label} · {anchor ? `anchored to ${anchor.title}` : meta.policy}</span>{busy && run ? <button className="stop-button" onClick={onCancel}>Stop</button> : <button className="send-button" onClick={onRun} disabled={!project || busy || !provider?.installed}>Run with {meta.label} ↗</button>}</div></footer></aside>;
}

/**
 * Live streaming log viewer showing streaming output during graph generation or room decomposition.
 */
function GraphGenerationStream({ transcript, expansion }: { transcript: string; expansion: boolean }) {
  const titles = extractStreamValues(transcript, "title").slice(0, 8);
  const tail = transcript.slice(-900);
  return <div className="graph-stream"><header><span>LIVE STRUCTURED STREAM</span><b>{transcript.length.toLocaleString()} chars</b></header>{titles.length > 0 && <div className="stream-discoveries">{titles.map((title, index) => <span key={`${title}-${index}`}><i />{!expansion && index === 0 ? `Graph · ${title}` : `Node · ${title}`}</span>)}</div>}<pre>{tail || "Waiting for the provider’s first structured output chunk…"}<i className="stream-cursor" /></pre></div>;
}

/**
 * Loads the flow graph for the given project from the desktop IPC bridge.
 *
 * @param projectId Unique ID of the project to load.
 * @param currentProjectId Accessor checking the currently active project ID to prevent race conditions.
 * @param setGraph State setter for the loaded flow graph.
 * @param setLoading State setter for the loading indicator.
 * @param setError State setter for reporting load failures.
 */
async function loadGraph(projectId: string, currentProjectId: () => string | null, setGraph: (value: GeneratedFlowGraph | null) => void, setLoading: (value: boolean) => void, setError: (value: string | null) => void) {
  setLoading(true);
  try { const value = await window.insightify.getProjectGraph(projectId); if (currentProjectId() === projectId) setGraph(value); }
  catch (reason) { setError(toMessage(reason)); }
  finally { if (currentProjectId() === projectId) setLoading(false); }
}

/**
 * Handles incoming graph generation events received from the background agent runtime.
 *
 * @param event The graph generation event payload.
 * @param currentProjectId ID of the currently selected project.
 * @param setGraph Setter to update the active flow graph.
 * @param setGenerating Setter for the global generation state.
 * @param setExpanding Setter for the room expansion scope ID.
 * @param setScope Setter for changing current room scope.
 * @param setBusy Setter for UI busy state.
 * @param setRun Setter for active agent run reference.
 * @param setEvents Setter for clearing/updating agent events.
 * @param setError Setter for reporting failure messages.
 */
function handleGraphEvent(event: GraphGenerationEvent, currentProjectId: string | null, setGraph: (value: GeneratedFlowGraph | null) => void, setGenerating: (value: boolean) => void, setExpanding: (value: string | null) => void, setScope: (value: string | null) => void, setBusy: (value: boolean) => void, setRun: (value: StartRunResult | null) => void, setEvents: (value: AgentEvent[]) => void, setError: (value: string | null) => void) {
  setGenerating(false); setExpanding(null); setBusy(false); setRun(null); setEvents([]);
  if (event.status === "completed") { if (event.value.projectId === currentProjectId) { setGraph(event.value); if (!event.scopeNodeId) setScope(null); } }
  else if (event.projectId === currentProjectId) setError(event.message);
}

/**
 * Responds to an interactive approval request from an agent run.
 *
 * @param project The active project summary.
 * @param requestId The ID of the approval request.
 * @param decision User decision ("accept" or "decline").
 * @param setError Error reporting callback.
 */
async function respond(project: ProjectSummary | null, requestId: string, decision: "accept" | "decline", setError: (value: string | null) => void) {
  if (!project) return;
  try { await window.insightify.respondToAgentApproval({ provider: "codex", projectId: project.id, requestId, decision }); }
  catch (reason) { setError(toMessage(reason)); }
}

/**
 * Finds the direct child node ID within `scopeId` that lies on the ancestral path from `from`.
 *
 * @param graph The complete generated flow graph.
 * @param from The starting node ID.
 * @param scopeId The target ancestor scope ID.
 * @returns The direct child ID within the target scope, or null.
 */
function ancestorWithin(graph: GeneratedFlowGraph | null, from: string | null, scopeId: string | null): string | null {
  const visited = new Set<string>();
  let cursor = from;
  while (cursor && !visited.has(cursor)) {
    visited.add(cursor);
    const node = graph?.graph.nodes.find((item) => item.id === cursor);
    if (!node) return null;
    if (node.parentId === scopeId) return node.id;
    cursor = node.parentId;
  }
  return null;
}

/**
 * Builds an array of ancestor flow nodes representing the breadcrumb path to the given scope.
 *
 * @param graph The generated flow graph.
 * @param scopeId The target scope node ID.
 * @returns Ordered list of ancestor nodes from root down to the target scope.
 */
function buildScopePath(graph: GeneratedFlowGraph | null, scopeId: string | null): FlowNode[] {
  if (!graph || !scopeId) return [];
  const result: FlowNode[] = [];
  let cursor: string | null = scopeId;
  const visited = new Set<string>();
  while (cursor && !visited.has(cursor)) { visited.add(cursor); const node = graph.graph.nodes.find((item) => item.id === cursor); if (!node) break; result.unshift(node); cursor = node.parentId; }
  return result;
}

/**
 * Computes the set of all descendant node IDs (children, grandchildren, etc.) for a root node.
 *
 * @param nodes All nodes in the flow graph.
 * @param rootId The root node ID to collect descendants for.
 * @returns Set containing the rootId and all its recursive descendant IDs.
 */
function descendantIds(nodes: FlowNode[], rootId: string): Set<string> {
  const result = new Set([rootId]);
  let changed = true;
  while (changed) { changed = false; for (const node of nodes) if (node.parentId && result.has(node.parentId) && !result.has(node.id)) { result.add(node.id); changed = true; } }
  return result;
}

/** Default empty preview descriptor. */
const emptyPreview: PortalPreview = { nodes: [], edges: [], childCount: 0, descendantCount: 0, hiddenCount: 0 };

/** Type guard verifying that a projected edge connects two defined node endpoints in the active room. */
function isRoomEdge(edge: ProjectedFlowEdge): edge is RoomEdge { return edge.source !== null && edge.target !== null; }

/** Converts a room edge into a standard flow edge descriptor. */
function toFlowEdge(edge: RoomEdge): FlowEdge { return { source: edge.source, target: edge.target, label: edge.labels[0] ?? "" }; }

/** Determines whether a given node has input and output edge or boundary port connections. */
function connectionSides(nodeId: string, edges: RoomEdge[], ports: ScopeBoundaryPort[]): { input: boolean; output: boolean } {
  return {
    input: edges.some((edge) => edge.target === nodeId) || ports.some((port) => port.side === "input" && port.nodeId === nodeId),
    output: edges.some((edge) => edge.source === nodeId) || ports.some((port) => port.side === "output" && port.nodeId === nodeId),
  };
}

/** Formats a unique React key for a boundary port chip. */
function portKey(port: ScopeBoundaryPort): string { return `${port.side}-${port.nodeId}`; }

/** Metric functions and half-dimensions for projecting stage coordinates onto the canvas frame. */
type FrameProjection = { x(value: number): number; y(value: number): number; cardHalfWidth: number; cardHalfHeight: number };

/** Layout position information for boundary port chips and connecting links. */
type PlacedPort = { port: ScopeBoundaryPort; chipX: number; chipY: number; fromX: number; fromY: number; toX: number; toY: number };

/**
 * Computes frame projection transforms mapping stage percentage coordinates to canvas frame positions.
 * `scale` is the scale actually applied to the stage - the fit multiplied by the
 * user's zoom - so chips and labels track the Portals at every zoom step.
 *
 * @param stageWidth Natural width of the stage.
 * @param stageHeight Natural height of the stage.
 * @param scale Total zoom and fit scale factor.
 * @param frame Canvas viewport dimensions.
 * @returns Projection helpers and card dimensions in frame percentage units.
 */
function frameProjection(stageWidth: number, stageHeight: number, scale: number, frame: { width: number; height: number }): FrameProjection {
  const width = Math.max(1, frame.width);
  const height = Math.max(1, frame.height);
  return {
    x: (value) => 50 + (value - 50) * (stageWidth * scale) / width,
    y: (value) => 50 + (value - 50) * (stageHeight * scale) / height,
    cardHalfWidth: ((PORTAL_CARD_WIDTH / 2) * scale * 100) / width,
    cardHalfHeight: (98 * scale * 100) / height,
  };
}

/**
 * Computes positioning for boundary chips and rail lines above the active flow.
 * Boundary chips live in the margin above the flow, not beside it: the gap next
 * to a Portal is never wide enough for the name of what lies outside.
 *
 * @param ports Boundary ports connected to nodes in this room.
 * @param nodes Screen-positioned nodes in the active room.
 * @param project Frame projection transform.
 * @returns Array of placed port layouts and link coordinates.
 */
function layoutBoundaryRail(ports: ScopeBoundaryPort[], nodes: Array<{ id: string; x: number; y: number }>, project: FrameProjection): PlacedPort[] {
  const byId = new Map(nodes.map((node) => [node.id, node]));
  const bandTop = Math.min(...nodes.map((node) => project.y(node.y))) - project.cardHalfHeight;
  const placed: PlacedPort[] = [];
  for (const side of ["input", "output"] as const) {
    const chipX = side === "input" ? 4 : 96;
    ports
      .filter((port) => port.side === side && byId.has(port.nodeId))
      .sort((left, right) => byId.get(left.nodeId)!.x - byId.get(right.nodeId)!.x)
      .forEach((port, index) => {
        const node = byId.get(port.nodeId)!;
        placed.push({
          port,
          chipX,
          chipY: Math.max(4, bandTop - 5 - index * 4),
          fromX: chipX,
          fromY: Math.max(4, bandTop - 5 - index * 4),
          toX: project.x(node.x) + (side === "input" ? -project.cardHalfWidth : project.cardHalfWidth),
          toY: project.y(node.y),
        });
      });
  }
  return placed;
}

/** Clears all active dive animation timeouts. */
function clearDive(timers: { current: ReturnType<typeof setTimeout>[] }): void {
  timers.current.forEach(clearTimeout);
  timers.current = [];
}

/** Checks whether the client environment requests reduced motion. */
function prefersReducedMotion(): boolean {
  return typeof window.matchMedia === "function" && window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}

/** Parses newline-separated evidence string into clean path strings. */
function parseEvidence(value: string): string[] { return value.split("\n").map((item) => item.trim()).filter(Boolean).slice(0, 4).map((item) => item.slice(0, 240)); }

/** Resolves the display title for a node ID from a list of nodes, falling back to the ID. */
function nodeTitle(nodes: FlowNode[], id: string): string { return nodes.find((node) => node.id === id)?.title ?? id; }

/** Determines the user-facing status description of the current graph generation phase. */
function generationPhase(events: AgentEvent[], transcript: string): string {
  if (transcript) return "Streaming structured graph";
  if (events.some((event) => event.type === "run.started")) return "Analyzing project snapshot";
  if (events.some((event) => event.type === "provider.connected")) return "Provider connected";
  return "Preparing safe snapshot";
}

/** Extracts string property values for a given key from raw streaming JSON text. */
function extractStreamValues(text: string, key: string): string[] {
  const values: string[] = [];
  const pattern = new RegExp(`"${key}"\\s*:\\s*"((?:\\\\.|[^"\\\\])*)"`, "g");
  for (const match of text.matchAll(pattern)) {
    try { values.push(JSON.parse(`"${match[1]}"`) as string); } catch { values.push(match[1]); }
  }
  return values;
}

/** Returns the decorative icon character for a given node kind. */
function kindIcon(kind: string): string { return ({ room: "↳", process: "→", decision: "◇", data: "▤", external: "↗" } as Record<string, string>)[kind] ?? "·"; }

/** Clamps a numeric value between minimum and maximum bounds. */
function clamp(value: number, minimum: number, maximum: number): number { return Math.min(maximum, Math.max(minimum, value)); }

/** Converts an unknown error value into a readable string message. */
function toMessage(reason: unknown): string { return reason instanceof Error ? reason.message : String(reason); }

/** Type guard checking if an AgentEvent is an approval request. */
function isApprovalRequested(event: AgentEvent): event is ApprovalRequestedEvent { return event.type === "approval.requested"; }

/** Type guard checking if an AgentEvent is an approval resolution. */
function isApprovalResolved(event: AgentEvent): event is ApprovalResolvedEvent { return event.type === "approval.resolved"; }
