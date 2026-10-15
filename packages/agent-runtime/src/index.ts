/**
 * Supported provider backends for agent execution.
 */
export type AgentProviderKind = "codex" | "antigravity-cli" | "antigravity-sdk";

/**
 * Feature matrix representing the capabilities of a specific agent provider runtime.
 *
 * @property managedSubscriptionLogin Whether the provider can authenticate through a
 *   managed subscription rather than an externally supplied API key.
 * @property accountStatus Whether the provider exposes structured account or login status.
 * @property rateLimits How rate-limit information is represented by the provider.
 * @property persistentThreads Whether conversations can be resumed by thread ID.
 * @property forkThread Whether an existing thread can be forked into a new run.
 * @property resumeThread Whether a previously interrupted thread can be resumed.
 * @property interactiveApprovals Whether the provider can pause for command/file approvals.
 * @property sandboxPolicy How sandbox restrictions are configured or reported.
 * @property structuredOutput Whether the provider can enforce a JSON/schema output contract.
 * @property streamedToolEvents Whether tool lifecycle events are emitted incrementally.
 * @property cancelTurn Whether an in-flight turn can be cancelled.
 */
export type AgentCapabilities = {
  managedSubscriptionLogin: boolean;
  accountStatus: boolean;
  rateLimits: "structured" | "text" | "none";
  persistentThreads: boolean;
  forkThread: boolean;
  resumeThread: boolean;
  interactiveApprovals: boolean;
  sandboxPolicy: "structured" | "provider-config" | "none";
  structuredOutput: boolean;
  streamedToolEvents: boolean;
  cancelTurn: boolean;
};

/**
 * Installation and version detection status for an agent provider executable.
 *
 * @property provider Stable provider identifier used in persisted events.
 * @property installed Whether the executable was found and responded to a probe.
 * @property version Detected provider version, or null when unavailable.
 * @property error Human-readable probe failure, or null when probing succeeded.
 * @property capabilities Capability matrix used by the UI and runtime feature gates.
 */
export type ProviderInstallation = {
  provider: AgentProviderKind;
  installed: boolean;
  version: string | null;
  error: string | null;
  capabilities: AgentCapabilities;
};

/**
 * Decisions available when responding to an interactive approval request from an agent.
 */
export type ApprovalDecision = "accept" | "acceptForSession" | "decline" | "cancel";

/**
 * Base metadata common to all streaming agent events.
 *
 * @property provider Provider that emitted the event.
 * @property projectId Project whose agent run produced the event.
 * @property sequence Monotonically increasing sequence number within the client process.
 * @property timestamp ISO-8601 timestamp assigned when the event is emitted.
 * @property threadId Provider conversation/thread identifier, when known.
 * @property runId Provider turn/run identifier, when known.
 */
type AgentEventBase = {
  provider: AgentProviderKind;
  projectId: string;
  sequence: number;
  timestamp: string;
  threadId: string | null;
  runId: string | null;
};

/**
 * Discriminated union of all streaming agent lifecycle, content, tool, and diagnostic events.
 */
export type AgentEvent =
  | (AgentEventBase & { type: "provider.connected"; version: string | null })
  | (AgentEventBase & { type: "provider.disconnected"; reason: string })
  | (AgentEventBase & { type: "run.started" })
  | (AgentEventBase & { type: "assistant.delta"; text: string })
  | (AgentEventBase & { type: "reasoning.delta"; text: string })
  | (AgentEventBase & { type: "tool.started"; itemId: string; tool: string; summary: string })
  | (AgentEventBase & { type: "tool.completed"; itemId: string; tool: string; success: boolean })
  | (AgentEventBase & {
      type: "approval.requested";
      requestId: string;
      itemId: string;
      approvalKind: "command" | "file-change";
      reason: string | null;
      command: string | null;
      cwd: string | null;
    })
  | (AgentEventBase & { type: "approval.resolved"; requestId: string })
  | (AgentEventBase & { type: "diff.updated"; diff: string })
  | (AgentEventBase & { type: "usage.updated"; usage: unknown })
  | (AgentEventBase & { type: "warning"; message: string })
  | (AgentEventBase & { type: "run.completed"; status: "completed" | "interrupted" })
  | (AgentEventBase & { type: "run.failed"; message: string; code: string | null })
  | (AgentEventBase & { type: "provider.event"; method: string; payload: unknown });

/**
 * Returns the default capability configuration for the Codex agent provider.
 * @returns The capability matrix supported by Codex.
 */
export function codexCapabilities(): AgentCapabilities {
  return {
    managedSubscriptionLogin: true,
    accountStatus: true,
    rateLimits: "structured",
    persistentThreads: true,
    forkThread: true,
    resumeThread: true,
    interactiveApprovals: true,
    sandboxPolicy: "structured",
    structuredOutput: true,
    streamedToolEvents: true,
    cancelTurn: true,
  };
}

/**
 * Returns the default capability configuration for the Antigravity CLI agent provider.
 * @returns The capability matrix supported by Antigravity CLI.
 */
export function antigravityCliCapabilities(): AgentCapabilities {
  return {
    managedSubscriptionLogin: true,
    accountStatus: false,
    rateLimits: "text",
    persistentThreads: false,
    forkThread: false,
    resumeThread: true,
    interactiveApprovals: false,
    sandboxPolicy: "provider-config",
    structuredOutput: true,
    streamedToolEvents: true,
    cancelTurn: true,
  };
}
