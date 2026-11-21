import type { AgentEvent } from "@insightify/agent-runtime";
import { isRecord, type JsonRpcNotification, type JsonRpcServerRequest } from "./protocol.js";

/**
 * Contextual metadata required to normalize events for a specific project run.
 */
type NormalizeContext = {
  projectId: string;
  sequence: number;
};

/** Extracts string value or returns null if not a string. */
function text(value: unknown): string | null {
  return typeof value === "string" ? value : null;
}

/** Casts or normalizes a value to an object record. */
function record(value: unknown): Record<string, unknown> {
  return isRecord(value) ? value : {};
}

/** Constructs the common base fields for a normalized AgentEvent. */
function base(context: NormalizeContext, params: Record<string, unknown>) {
  const turn = record(params.turn);
  return {
    provider: "codex" as const,
    projectId: context.projectId,
    sequence: context.sequence,
    timestamp: new Date().toISOString(),
    threadId: text(params.threadId),
    runId: text(params.turnId) ?? text(turn.id),
  };
}

/** Extracts tool identifier from a Codex item object. */
function itemTool(item: Record<string, unknown>): string {
  return text(item.type) ?? "unknown";
}

/**
 * Normalizes a raw Codex JSON-RPC notification message into standard domain AgentEvents.
 * @param message - The raw JSON-RPC notification from Codex app-server.
 * @param context - Project and sequence context.
 * @returns Array of normalized AgentEvent objects.
 */
export function normalizeCodexNotification(
  message: JsonRpcNotification,
  context: NormalizeContext,
): AgentEvent[] {
  const params = record(message.params);
  const eventBase = base(context, params);

  switch (message.method) {
    case "turn/started":
      return [{ ...eventBase, type: "run.started" }];
    case "item/agentMessage/delta":
      return [{ ...eventBase, type: "assistant.delta", text: text(params.delta) ?? "" }];
    case "item/reasoning/summaryTextDelta":
      return [{ ...eventBase, type: "reasoning.delta", text: text(params.delta) ?? "" }];
    case "turn/diff/updated":
      return [{ ...eventBase, type: "diff.updated", diff: text(params.diff) ?? "" }];
    case "thread/tokenUsage/updated":
    case "account/rateLimits/updated":
      return [{ ...eventBase, type: "usage.updated", usage: message.params }];
    case "warning":
    case "configWarning":
      return [{ ...eventBase, type: "warning", message: text(params.message) ?? text(params.summary) ?? "Codex warning" }];
    case "serverRequest/resolved":
      return [{ ...eventBase, type: "approval.resolved", requestId: String(params.requestId ?? "") }];
    case "item/started": {
      const item = record(params.item);
      const tool = itemTool(item);
      if (tool === "commandExecution" || tool === "fileChange" || tool === "mcpToolCall" || tool === "dynamicToolCall") {
        return [{
          ...eventBase,
          type: "tool.started",
          itemId: text(item.id) ?? "unknown",
          tool,
          summary: text(item.command) ?? text(item.tool) ?? tool,
        }];
      }
      return [];
    }
    case "item/completed": {
      const item = record(params.item);
      const tool = itemTool(item);
      if (tool === "commandExecution" || tool === "fileChange" || tool === "mcpToolCall" || tool === "dynamicToolCall") {
        const status = text(item.status);
        return [{
          ...eventBase,
          type: "tool.completed",
          itemId: text(item.id) ?? "unknown",
          tool,
          success: status === "completed",
        }];
      }
      return [];
    }
    case "turn/completed": {
      const turn = record(params.turn);
      const status = text(turn.status);
      if (status === "failed") {
        const error = record(turn.error);
        const info = record(error.codexErrorInfo);
        return [{
          ...eventBase,
          type: "run.failed",
          message: text(error.message) ?? "Codex run failed",
          code: text(info.type) ?? text(info.code),
        }];
      }
      return [{ ...eventBase, type: "run.completed", status: status === "interrupted" ? "interrupted" : "completed" }];
    }
    case "error": {
      const error = record(params.error);
      const info = record(error.codexErrorInfo);
      return [{
        ...eventBase,
        type: "run.failed",
        message: text(error.message) ?? "Codex error",
        code: text(info.type) ?? text(info.code),
      }];
    }
    default:
      return [{ ...eventBase, type: "provider.event", method: message.method, payload: message.params }];
  }
}

/**
 * Normalizes a server-initiated request from Codex (e.g. command or file approval) into an approval.requested AgentEvent.
 * @param message - The server-initiated JSON-RPC request.
 * @param context - Project and sequence context.
 * @returns Array of normalized AgentEvent objects.
 */
export function normalizeCodexServerRequest(
  message: JsonRpcServerRequest,
  context: NormalizeContext,
): AgentEvent[] {
  const params = record(message.params);
  const eventBase = base(context, params);
  const isCommand = message.method === "item/commandExecution/requestApproval";
  const isFile = message.method === "item/fileChange/requestApproval";
  if (!isCommand && !isFile) {
    return [{ ...eventBase, type: "provider.event", method: message.method, payload: message.params }];
  }

  const commandValue = params.command;
  const command = Array.isArray(commandValue)
    ? commandValue.filter((part): part is string => typeof part === "string").join(" ")
    : text(commandValue);

  return [{
    ...eventBase,
    type: "approval.requested",
    requestId: String(message.id),
    itemId: text(params.itemId) ?? "unknown",
    approvalKind: isCommand ? "command" : "file-change",
    reason: text(params.reason),
    command,
    cwd: text(params.cwd),
  }];
}
