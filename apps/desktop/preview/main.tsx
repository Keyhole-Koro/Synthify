/**
 * Standalone browser preview harness.
 * Mounts the real App against a stubbed preload bridge, so the FlowFold canvas
 * can be driven, previewed, and tested without requiring an Electron shell or CLI agents.
 *
 * @module
 */
import { createRoot } from "react-dom/client";
import type { GeneratedFlowGraph } from "@insightify/graph-domain";
import type { AgentCapabilities, ProviderInstallation } from "@insightify/agent-runtime";
import type { InsightifyDesktopApi, ProjectSummary } from "@insightify/desktop-bridge";
import { App } from "../src/renderer/App.js";
import { previewGraph } from "./fixture.js";
import "../src/renderer/styles.css";

/** Mock project summary for preview mode. */
const project: ProjectSummary = { id: previewGraph.projectId, displayName: "Insightify", lastOpenedAt: previewGraph.generatedAt };
let stored: GeneratedFlowGraph = previewGraph;

/** Mock agent capabilities supported in preview mode. */
const capabilities: AgentCapabilities = {
  managedSubscriptionLogin: true, accountStatus: true, rateLimits: "structured", persistentThreads: true,
  forkThread: true, resumeThread: true, interactiveApprovals: true, sandboxPolicy: "structured",
  structuredOutput: true, streamedToolEvents: true, cancelTurn: true,
};

/** Helper factory for simulated provider installation descriptors. */
const installed = (provider: ProviderInstallation["provider"]): ProviderInstallation => ({ provider, installed: true, version: "preview", error: null, capabilities });

/** Mock desktop bridge API implementation for browser previewing. */
const api: InsightifyDesktopApi = {
  pickProject: async () => project,
  listProjects: async () => [project],
  getProjectGraph: async () => stored,
  saveProjectGraph: async (value) => { stored = value; return value; },
  probeProviders: async () => [installed("antigravity-cli"), installed("codex")],
  startAgentRun: async () => ({ threadId: "preview-thread", runId: "preview-run" }),
  generateFlowGraph: async () => ({ threadId: "preview-thread", runId: "preview-run" }),
  cancelAgentRun: async () => {},
  respondToAgentApproval: async () => {},
  onAgentEvent: () => () => {},
  onGraphGeneration: () => () => {},
};

(window as unknown as { insightify: InsightifyDesktopApi }).insightify = api;
createRoot(document.getElementById("root")!).render(<App />);

