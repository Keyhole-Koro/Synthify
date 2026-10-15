/**
 * Preload script exposing the secure desktop bridge API to the renderer process.
 * Mediates communication between web views and Electron main IPC channels.
 *
 * @module
 */
import { contextBridge, ipcRenderer } from "electron";
import { IPC_CHANNELS, type GraphGenerationEvent, type InsightifyDesktopApi } from "@insightify/desktop-bridge";
import type { AgentEvent } from "@insightify/agent-runtime";

/**
 * Concrete implementation of InsightifyDesktopApi exposed through Electron's context bridge.
 */
const api: InsightifyDesktopApi = {
  pickProject: () => ipcRenderer.invoke(IPC_CHANNELS.projectPick),
  listProjects: () => ipcRenderer.invoke(IPC_CHANNELS.projectList),
  getProjectGraph: (projectId) => ipcRenderer.invoke(IPC_CHANNELS.projectGraphGet, { projectId }),
  saveProjectGraph: (value) => ipcRenderer.invoke(IPC_CHANNELS.projectGraphSave, value),
  probeProviders: () => ipcRenderer.invoke(IPC_CHANNELS.providersProbe),
  startAgentRun: (input) => ipcRenderer.invoke(IPC_CHANNELS.agentStartRun, input),
  generateFlowGraph: (input) => ipcRenderer.invoke(IPC_CHANNELS.graphGenerate, input),
  cancelAgentRun: (input) => ipcRenderer.invoke(IPC_CHANNELS.agentCancelRun, input),
  respondToAgentApproval: (input) => ipcRenderer.invoke(IPC_CHANNELS.agentApprovalRespond, input),
  onAgentEvent: (listener) => {
    const handler = (_event: Electron.IpcRendererEvent, payload: AgentEvent) => listener(payload);
    ipcRenderer.on(IPC_CHANNELS.agentEvent, handler);
    return () => ipcRenderer.off(IPC_CHANNELS.agentEvent, handler);
  },
  onGraphGeneration: (listener) => {
    const handler = (_event: Electron.IpcRendererEvent, payload: GraphGenerationEvent) => listener(payload);
    ipcRenderer.on(IPC_CHANNELS.graphGeneration, handler);
    return () => ipcRenderer.off(IPC_CHANNELS.graphGeneration, handler);
  },
};

contextBridge.exposeInMainWorld("insightify", api);
