import { dialog, ipcMain, type BrowserWindow, type OpenDialogOptions } from "electron";
import {
  approvalResponseSchema,
  cancelAgentRunInputSchema,
  generateFlowGraphInputSchema,
  IPC_CHANNELS,
  startAgentRunInputSchema,
  projectGraphInputSchema,
  saveProjectGraphInputSchema,
} from "@insightify/desktop-bridge";
import type { AgentRuntimeManager } from "./agent-runtime-manager.js";
import type { ProjectRepository } from "./project-repository.js";

/**
 * Options for configuring and registering IPC handlers for the desktop main process.
 */
type RegisterIpcOptions = {
  /** Repository for managing local project snapshots and graph data. */
  projects: ProjectRepository;
  /** Manager controlling background CLI agent runtimes and streaming. */
  agents: AgentRuntimeManager;
  /** Factory function returning the current active browser window instance or null. */
  window: () => BrowserWindow | null;
};

/**
 * Registers IPC handlers for project management, graph persistence, and agent execution.
 *
 * @param options Configuration options containing repositories, agent runtimes, and window access.
 * @returns A teardown function that removes registered IPC handlers upon disposal.
 */
export function registerIpc(options: RegisterIpcOptions): () => void {
  ipcMain.handle(IPC_CHANNELS.projectPick, async () => {
    const dialogOptions: OpenDialogOptions = {
      title: "Open an Insightify project",
      properties: ["openDirectory"],
    };
    const window = options.window();
    const result = window
      ? await dialog.showOpenDialog(window, dialogOptions)
      : await dialog.showOpenDialog(dialogOptions);
    if (result.canceled || result.filePaths.length === 0) return null;
    const project = options.projects.openPath(result.filePaths[0]);
    return {
      id: project.id,
      displayName: project.displayName,
      lastOpenedAt: project.lastOpenedAt,
    };
  });

  ipcMain.handle(IPC_CHANNELS.projectList, () => options.projects.list());
  ipcMain.handle(IPC_CHANNELS.projectGraphGet, (_event, raw: unknown) => {
    const input = projectGraphInputSchema.parse(raw);
    return options.projects.getGraph(input.projectId);
  });
  ipcMain.handle(IPC_CHANNELS.projectGraphSave, (_event, raw: unknown) => {
    const value = saveProjectGraphInputSchema.parse(raw);
    if (!options.projects.resolve(value.projectId)) throw new Error("Unknown project id");
    options.projects.saveGraph(value);
    return value;
  });
  ipcMain.handle(IPC_CHANNELS.providersProbe, () => options.agents.probeProviders());
  ipcMain.handle(IPC_CHANNELS.agentStartRun, async (_event, raw: unknown) => {
    const input = startAgentRunInputSchema.parse(raw);
    return options.agents.startRun(input.provider, input.projectId, input.prompt);
  });
  ipcMain.handle(IPC_CHANNELS.graphGenerate, async (_event, raw: unknown) => {
    const input = generateFlowGraphInputSchema.parse(raw);
    return options.agents.generateGraph(input.provider, input.projectId, input.scopeNodeId);
  });
  ipcMain.handle(IPC_CHANNELS.agentCancelRun, async (_event, raw: unknown) => {
    const input = cancelAgentRunInputSchema.parse(raw);
    await options.agents.cancelRun(input.provider, input.projectId, input.threadId, input.runId);
  });
  ipcMain.handle(IPC_CHANNELS.agentApprovalRespond, async (_event, raw: unknown) => {
    const input = approvalResponseSchema.parse(raw);
    await options.agents.respondToApproval(input.provider, input.projectId, input.requestId, input.decision);
  });

  return () => {
    for (const channel of Object.values(IPC_CHANNELS)) {
      if (channel !== IPC_CHANNELS.agentEvent && channel !== IPC_CHANNELS.graphGeneration) {
        ipcMain.removeHandler(channel);
      }
    }
  };
}
