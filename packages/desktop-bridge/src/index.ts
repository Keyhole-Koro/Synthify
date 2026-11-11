import type { AgentEvent, ApprovalDecision, ProviderInstallation } from "@insightify/agent-runtime";
import { generatedFlowGraphSchema, type GeneratedFlowGraph } from "@insightify/graph-domain";
import { z } from "zod";

/**
 * Electron IPC channel name constants for communication between main and renderer processes.
 */
export const IPC_CHANNELS = {
  projectPick: "insightify:project:pick",
  projectList: "insightify:project:list",
  projectGraphGet: "insightify:project:graph:get",
  projectGraphSave: "insightify:project:graph:save",
  graphGenerate: "insightify:graph:generate",
  graphGeneration: "insightify:graph:generation",
  providersProbe: "insightify:providers:probe",
  agentStartRun: "insightify:agent:run:start",
  agentCancelRun: "insightify:agent:run:cancel",
  agentApprovalRespond: "insightify:agent:approval:respond",
  agentEvent: "insightify:agent:event",
} as const;

/** UUID schema for project identifiers exchanged across the Electron IPC boundary. */
export const projectIdSchema = z.string().uuid();
/** Schema for the executable provider identifiers supported by the desktop bridge. */
export const executableAgentProviderSchema = z.enum(["codex", "antigravity-cli"]);

/**
 * Validates the input for starting an agent run.
 *
 * @property provider Provider backend that will execute the prompt.
 * @property projectId UUID of the project whose files are available to the agent.
 * @property prompt Non-empty user instruction, limited to 100,000 characters.
 */
export const startAgentRunInputSchema = z.object({
  provider: executableAgentProviderSchema,
  projectId: projectIdSchema,
  prompt: z.string().trim().min(1).max(100_000),
});

/** Validates the project identifier used by graph read/write IPC calls. */
export const projectGraphInputSchema = z.object({ projectId: projectIdSchema });
/** Validates a complete persisted graph bundle before it crosses IPC. */
export const saveProjectGraphInputSchema = generatedFlowGraphSchema;
/**
 * Validates the input for generating a graph or expanding a room.
 *
 * @property provider Provider backend that will perform generation.
 * @property projectId UUID of the project to inspect.
 * @property scopeNodeId Optional room node ID; when present, generation targets that room.
 */
export const generateFlowGraphInputSchema = z.object({
  provider: executableAgentProviderSchema,
  projectId: projectIdSchema,
  scopeNodeId: z.string().regex(/^[a-z][a-z0-9-]{0,39}$/).optional(),
});

/**
 * Validates the identifiers required to cancel an in-flight agent run.
 *
 * @property provider Provider that owns the run.
 * @property projectId Project associated with the run.
 * @property threadId Provider conversation identifier.
 * @property runId Provider turn/run identifier.
 */
export const cancelAgentRunInputSchema = z.object({
  provider: executableAgentProviderSchema,
  projectId: projectIdSchema,
  threadId: z.string().min(1),
  runId: z.string().min(1),
});

/**
 * Validates a renderer response to a pending approval request.
 *
 * @property provider Provider that issued the request.
 * @property projectId Project associated with the request.
 * @property requestId Provider approval request identifier.
 * @property decision User decision to accept, decline, or cancel the request.
 */
export const approvalResponseSchema = z.object({
  provider: executableAgentProviderSchema,
  projectId: projectIdSchema,
  requestId: z.string().min(1),
  decision: z.enum(["accept", "acceptForSession", "decline", "cancel"]),
});

/** Executable agent provider backend identifier. */
export type ExecutableAgentProvider = z.infer<typeof executableAgentProviderSchema>;
/**
 * Parameters for initiating an agent run.
 *
 * @property provider Provider backend that receives the prompt.
 * @property projectId UUID of the project used as the agent working context.
 * @property prompt User instruction passed to the provider.
 */
export type StartAgentRunInput = z.infer<typeof startAgentRunInputSchema>;
/**
 * Parameters for cancelling an in-flight agent run.
 *
 * @property provider Provider that owns the run.
 * @property projectId Project associated with the run.
 * @property threadId Conversation identifier.
 * @property runId Turn/run identifier to cancel.
 */
export type CancelAgentRunInput = z.infer<typeof cancelAgentRunInputSchema>;
/**
 * Response payload when resolving an interactive approval request.
 *
 * @property provider Provider that issued the approval request.
 * @property projectId Project associated with the request.
 * @property requestId Request identifier supplied by the provider.
 * @property decision Decision applied to the pending command or file change.
 */
export type ApprovalResponse = z.infer<typeof approvalResponseSchema> & { decision: ApprovalDecision };
/**
 * Input parameters for triggering flow graph generation via agent.
 *
 * @property provider Provider backend used to generate the graph.
 * @property projectId Project to snapshot and analyze.
 * @property scopeNodeId Optional room ID for a scoped expansion.
 */
export type GenerateFlowGraphInput = z.infer<typeof generateFlowGraphInputSchema>;

/**
 * Event notifying renderer of flow graph generation success or failure.
 *
 * The completed variant contains the validated persisted graph bundle. The failed variant
 * retains provider/project context so the renderer can show a useful error without guessing
 * which run failed.
 */
export type GraphGenerationEvent =
  | { status: "completed"; value: GeneratedFlowGraph; scopeNodeId?: string }
  | { status: "failed"; projectId: string; provider: ExecutableAgentProvider; scopeNodeId?: string; message: string };

/**
 * Summary metadata of a recently opened project.
 *
 * @property id Stable project identifier.
 * @property displayName User-facing project name.
 * @property lastOpenedAt ISO-8601 timestamp of the last successful open.
 */
export type ProjectSummary = {
  id: string;
  displayName: string;
  lastOpenedAt: string;
};

/**
 * Identifiers returned upon successfully launching an agent run.
 *
 * @property threadId Provider conversation identifier used by subsequent events.
 * @property runId Provider turn/run identifier used for cancellation.
 */
export type StartRunResult = {
  threadId: string;
  runId: string;
};

/**
 * Desktop bridge API exposed to the Electron renderer process via preload context bridge.
 */
export interface InsightifyDesktopApi {
  /**
   * Prompts the user to select a project directory and registers it for future sessions.
   * @returns The registered project summary, or null when the dialog is cancelled.
   */
  pickProject(): Promise<ProjectSummary | null>;
  /**
   * Lists all projects stored in the desktop database.
   * @returns Projects ordered by their repository-defined recency order.
   */
  listProjects(): Promise<ProjectSummary[]>;
  /**
   * Retrieves the persisted flow graph for a project, if present.
   * @param projectId UUID of the project to read.
   * @returns Validated graph bundle, or null when the project has not been analyzed.
   */
  getProjectGraph(projectId: string): Promise<GeneratedFlowGraph | null>;
  /**
   * Saves or updates the project's flow graph.
   * @param value Complete graph bundle produced by the provider and validated by the bridge.
   * @returns The persisted graph bundle after storage normalization.
   */
  saveProjectGraph(value: GeneratedFlowGraph): Promise<GeneratedFlowGraph>;
  /**
   * Probes installed agent providers on the host system.
   * @returns One installation status record per supported executable provider.
   */
  probeProviders(): Promise<ProviderInstallation[]>;
  /**
   * Starts an agent run in the specified project.
   * @param input Provider, project, and prompt validated by startAgentRunInputSchema.
   * @returns Provider thread/run identifiers used for event subscription and cancellation.
   */
  startAgentRun(input: StartAgentRunInput): Promise<StartRunResult>;
  /**
   * Generates a flow graph using an agent for the project.
   * @param input Provider/project context and optional room scope.
   * @returns Identifiers for the asynchronous generation run.
   */
  generateFlowGraph(input: GenerateFlowGraphInput): Promise<StartRunResult>;
  /**
   * Cancels an active agent run.
   * @param input Provider, project, thread, and run identifiers for the target.
   * @returns Promise fulfilled after the cancellation request has been dispatched.
   */
  cancelAgentRun(input: CancelAgentRunInput): Promise<void>;
  /**
   * Submits an approval or rejection response for a pending agent prompt.
   * @param input Provider approval context and the user's decision.
   * @returns Promise fulfilled after the response has been sent to the main process.
   */
  respondToAgentApproval(input: ApprovalResponse): Promise<void>;
  /**
   * Subscribes to streaming agent events from the main process.
   * @param listener Callback invoked for provider, run, tool, approval, and diagnostic events.
   * @returns Unsubscribe function that removes the listener.
   */
  onAgentEvent(listener: (event: AgentEvent) => void): () => void;
  /**
   * Subscribes to graph generation progress and completion events.
   * @param listener Callback invoked with completed or failed generation events.
   * @returns Unsubscribe function that removes the listener.
   */
  onGraphGeneration(listener: (event: GraphGenerationEvent) => void): () => void;
}
