import type { BrowserWindow } from "electron";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { AntigravityCliClient, probeAntigravity } from "@insightify/agent-antigravity-cli";
import { CodexAppServerClient, probeCodex } from "@insightify/agent-codex";
import type { AgentEvent, ApprovalDecision, ProviderInstallation } from "@insightify/agent-runtime";
import { IPC_CHANNELS, type ExecutableAgentProvider, type StartRunResult } from "@insightify/desktop-bridge";
import {
  FLOW_GRAPH_JSON_SCHEMA,
  FLOW_GRAPH_EXPANSION_JSON_SCHEMA,
  applyScopeExpansion,
  balanceFlowGraphScopes,
  createDefaultGraphLayout,
  parseFlowGraph,
  parseFlowGraphText,
  parseFlowGraphExpansion,
  parseFlowGraphExpansionText,
  type FlowGraph,
  type GeneratedFlowGraph,
} from "@insightify/graph-domain";
import type { ProjectRepository } from "./project-repository.js";
import { buildFlowGraphExpansionPrompt, buildFlowGraphPrompt, buildProjectSnapshot } from "./project-snapshot.js";

type CodexSession = { client: CodexAppServerClient; unsubscribe: () => void };
type AntigravitySession = { client: AntigravityCliClient; unsubscribe: () => void };
type GraphGeneration = {
  projectId: string;
  provider: ExecutableAgentProvider;
  runId: string | null;
  snapshotHash: string;
  transcript: string;
  structuredOutput: unknown;
  temporaryDirectory: string | null;
  scopeNodeId: string | null;
  existingGraph: FlowGraph | null;
  existingLayout: GeneratedFlowGraph["layout"];
};

/**
 * Coordinates agent provider execution sessions (Codex app-server, Antigravity CLI)
 * and orchestrates project flow graph generation.
 */
export class AgentRuntimeManager {
  readonly #projects: ProjectRepository;
  readonly #window: () => BrowserWindow | null;
  readonly #codexSessions = new Map<string, CodexSession>();
  readonly #antigravitySessions = new Map<string, AntigravitySession>();
  readonly #installations = new Map<ExecutableAgentProvider, ProviderInstallation>();
  readonly #graphGenerations: GraphGeneration[] = [];

  /**
   * Constructs an AgentRuntimeManager.
   * @param projects - Project storage repository.
   * @param window - Accessor for the active Electron BrowserWindow.
   */
  constructor(projects: ProjectRepository, window: () => BrowserWindow | null) {
    this.#projects = projects;
    this.#window = window;
  }

  /**
   * Probes system installations and capabilities for all supported agent providers.
   * @returns Array of ProviderInstallation status objects.
   */
  async probeProviders(): Promise<ProviderInstallation[]> {
    const installations = await Promise.all([probeCodex(), probeAntigravity()]);
    for (const installation of installations) {
      if (installation.provider === "codex" || installation.provider === "antigravity-cli") {
        this.#installations.set(installation.provider, installation);
      }
    }
    return installations;
  }

  /**
   * Starts an interactive or one-shot agent run in the specified project.
   * @param provider - Agent provider to use.
   * @param projectId - Target project UUID.
   * @param prompt - Instruction prompt for the agent.
   * @returns StartRunResult containing threadId and runId.
   */
  async startRun(provider: ExecutableAgentProvider, projectId: string, prompt: string): Promise<StartRunResult> {
    if (provider === "codex") return (await this.#codexFor(projectId)).startRun(prompt);
    return (await this.#antigravityFor(projectId)).startRun(prompt);
  }

  /**
   * Initiates autonomous flow graph generation for a project snapshot, optionally expanding a specific room.
   * @param provider - Agent provider to use for generation.
   * @param projectId - Target project UUID.
   * @param scopeNodeId - Optional room node ID to expand instead of generating root graph.
   * @returns StartRunResult with execution identifiers.
   */
  async generateGraph(provider: ExecutableAgentProvider, projectId: string, scopeNodeId?: string): Promise<StartRunResult> {
    if (this.#graphGenerations.some((item) => item.projectId === projectId)) {
      throw new Error("A graph generation is already running for this project");
    }
    const project = this.#requireProject(projectId);
    const currentDocument = scopeNodeId ? this.#projects.getGraph(projectId) : null;
    if (scopeNodeId && !currentDocument) throw new Error("Generate the root FlowFold Graph before expanding a Room");
    const snapshot = await buildProjectSnapshot(project.canonicalPath);
    const prompt = scopeNodeId
      ? buildFlowGraphExpansionPrompt(snapshot, currentDocument!.graph, scopeNodeId)
      : buildFlowGraphPrompt(snapshot);
    const temporaryDirectory = await mkdtemp(path.join(tmpdir(), "insightify-graph-"));
    const generation: GraphGeneration = {
      projectId,
      provider,
      runId: null,
      snapshotHash: snapshot.hash,
      transcript: "",
      structuredOutput: null,
      temporaryDirectory,
      scopeNodeId: scopeNodeId ?? null,
      existingGraph: currentDocument?.graph ?? null,
      existingLayout: currentDocument?.layoutVersion === 2 ? currentDocument.layout : {},
    };
    this.#graphGenerations.push(generation);

    try {
      const handle = provider === "codex"
        ? await (await this.#codexFor(projectId)).startRun(prompt, {
            outputSchema: scopeNodeId ? FLOW_GRAPH_EXPANSION_JSON_SCHEMA : FLOW_GRAPH_JSON_SCHEMA,
          readOnly: true,
          cwd: temporaryDirectory,
          })
        : await (await this.#antigravityFor(projectId)).startRun(prompt, {
            cwd: temporaryDirectory ?? undefined,
            jsonSchema: scopeNodeId ? FLOW_GRAPH_EXPANSION_JSON_SCHEMA : FLOW_GRAPH_JSON_SCHEMA,
          });
      generation.runId = handle.runId;
      return handle;
    } catch (error) {
      this.#removeGeneration(generation);
      this.#sendGraphEvent({
        status: "failed",
        projectId,
        provider,
        ...(scopeNodeId ? { scopeNodeId } : {}),
        message: toMessage(error),
      });
      throw error;
    }
  }

  /**
   * Cancels an active agent run for the given provider and project.
   */
  async cancelRun(
    provider: ExecutableAgentProvider,
    projectId: string,
    threadId: string,
    runId: string,
  ): Promise<void> {
    if (provider === "codex") {
      await (await this.#codexFor(projectId)).cancelRun(threadId, runId);
      return;
    }
    await (await this.#antigravityFor(projectId)).cancelRun(runId);
  }

  /**
   * Forwards an interactive approval decision to a running Codex agent turn.
   */
  async respondToApproval(
    provider: ExecutableAgentProvider,
    projectId: string,
    requestId: string,
    decision: ApprovalDecision,
  ): Promise<void> {
    if (provider !== "codex") throw new Error("Antigravity CLI headless runs do not support interactive approvals");
    (await this.#codexFor(projectId)).respondToApproval(requestId, decision);
  }

  /**
   * Shuts down all active agent sessions and cleans up temporary generation workspaces.
   */
  async stop(): Promise<void> {
    const codex = [...this.#codexSessions.values()];
    const antigravity = [...this.#antigravitySessions.values()];
    this.#codexSessions.clear();
    this.#antigravitySessions.clear();
    for (const generation of [...this.#graphGenerations]) this.#removeGeneration(generation);
    await Promise.all([
      ...codex.map(async ({ client, unsubscribe }) => { unsubscribe(); await client.stop(); }),
      ...antigravity.map(async ({ client, unsubscribe }) => { unsubscribe(); await client.stop(); }),
    ]);
  }

  async #codexFor(projectId: string): Promise<CodexAppServerClient> {
    const existing = this.#codexSessions.get(projectId);
    if (existing) return existing.client;
    const project = this.#requireProject(projectId);
    const installation = await this.#installation("codex");
    const client = new CodexAppServerClient({ projectId, cwd: project.canonicalPath, version: installation.version });
    const unsubscribe = client.onEvent((event) => this.#forward(event));
    this.#codexSessions.set(projectId, { client, unsubscribe });
    return client;
  }

  async #antigravityFor(projectId: string): Promise<AntigravityCliClient> {
    const existing = this.#antigravitySessions.get(projectId);
    if (existing) return existing.client;
    const project = this.#requireProject(projectId);
    const installation = await this.#installation("antigravity-cli");
    const client = new AntigravityCliClient({ projectId, cwd: project.canonicalPath, version: installation.version });
    const unsubscribe = client.onEvent((event) => this.#forward(event));
    this.#antigravitySessions.set(projectId, { client, unsubscribe });
    return client;
  }

  #requireProject(projectId: string) {
    const project = this.#projects.resolve(projectId);
    if (!project) throw new Error("Unknown project id");
    return project;
  }

  async #installation(provider: ExecutableAgentProvider): Promise<ProviderInstallation> {
    const cached = this.#installations.get(provider);
    const installation = cached ?? (provider === "codex" ? await probeCodex() : await probeAntigravity());
    this.#installations.set(provider, installation);
    if (!installation.installed) {
      const label = provider === "codex" ? "Codex" : "Antigravity";
      throw new Error(installation.error ?? `${label} CLI is not installed`);
    }
    return installation;
  }

  #forward(event: AgentEvent): void {
    const generation = this.#graphGenerations.find((item) =>
      item.projectId === event.projectId
      && item.provider === event.provider
      && (item.runId === null || event.runId === item.runId),
    );
    if (generation) {
      if (event.type === "assistant.delta") generation.transcript += event.text;
      if (event.type === "provider.event" && event.method === "result") {
        const message = asRecord(event.payload);
        generation.structuredOutput = asRecord(message.result).structured_output ?? null;
      }
      if (event.type === "run.completed") this.#completeGeneration(generation, event.status);
      if (event.type === "run.failed") this.#failGeneration(generation, event.message);
    }
    const window = this.#window();
    if (!window || window.isDestroyed()) return;
    window.webContents.send(IPC_CHANNELS.agentEvent, event);
  }

  #completeGeneration(generation: GraphGeneration, status: "completed" | "interrupted"): void {
    if (status === "interrupted") {
      this.#failGeneration(generation, "Graph generation was interrupted");
      return;
    }
    try {
      const generatedGraph = generation.scopeNodeId && generation.existingGraph
        ? applyScopeExpansion(
            generation.existingGraph,
            generation.structuredOutput === null
              ? parseFlowGraphExpansionText(generation.transcript)
              : parseFlowGraphExpansion(generation.structuredOutput),
            generation.scopeNodeId,
          )
        : generation.structuredOutput === null
          ? parseFlowGraphText(generation.transcript)
          : parseFlowGraph(generation.structuredOutput);
      const graph = balanceFlowGraphScopes(generatedGraph);
      const value: GeneratedFlowGraph = {
        projectId: generation.projectId,
        provider: generation.provider,
        snapshotHash: generation.snapshotHash,
        generatedAt: new Date().toISOString(),
        graph,
        layout: createDefaultGraphLayout(graph, generation.existingLayout),
        layoutVersion: 2,
      };
      this.#projects.saveGraph(value);
      this.#sendGraphEvent({ status: "completed", value, ...(generation.scopeNodeId ? { scopeNodeId: generation.scopeNodeId } : {}) });
    } catch (error) {
      this.#sendGraphEvent({
        status: "failed",
        projectId: generation.projectId,
        provider: generation.provider,
        ...(generation.scopeNodeId ? { scopeNodeId: generation.scopeNodeId } : {}),
        message: `The provider returned an invalid FlowFold graph: ${toMessage(error)}`,
      });
    } finally {
      this.#removeGeneration(generation);
    }
  }

  #failGeneration(generation: GraphGeneration, message: string): void {
    this.#sendGraphEvent({
      status: "failed",
      projectId: generation.projectId,
      provider: generation.provider,
      ...(generation.scopeNodeId ? { scopeNodeId: generation.scopeNodeId } : {}),
      message,
    });
    this.#removeGeneration(generation);
  }

  #removeGeneration(generation: GraphGeneration): void {
    const index = this.#graphGenerations.indexOf(generation);
    if (index >= 0) this.#graphGenerations.splice(index, 1);
    if (generation.temporaryDirectory) {
      void rm(generation.temporaryDirectory, { recursive: true, force: true });
    }
  }

  #sendGraphEvent(event: import("@insightify/desktop-bridge").GraphGenerationEvent): void {
    const window = this.#window();
    if (!window || window.isDestroyed()) return;
    window.webContents.send(IPC_CHANNELS.graphGeneration, event);
  }
}

function asRecord(value: unknown): Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? value as Record<string, unknown>
    : {};
}

function toMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
