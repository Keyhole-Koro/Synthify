import type { AgentEvent } from "@insightify/agent-runtime";

/**
 * Contextual metadata required for normalizing events from an Antigravity CLI run.
 */
type NormalizeContext = {
  projectId: string;
  threadId: string;
  runId: string;
  nextSequence(): number;
};

/**
 * Parses a single JSON line string emitted by the Antigravity CLI process.
 * @param line - Raw line from stdout.
 * @returns Record representing the parsed JSON event.
 * @throws Error if the line is not a valid JSON object.
 */
export function parseAntigravityLine(line: string): Record<string, unknown> {
  const value: unknown = JSON.parse(line);
  if (!isRecord(value)) throw new Error("Antigravity CLI emitted a non-object JSON value");
  return value;
}

/**
 * Normalizes an Antigravity CLI event (e.g. step_update, result) into standardized AgentEvents.
 * @param message - The raw Antigravity CLI event record.
 * @param context - Execution context providing IDs and sequence generator.
 * @returns Array of normalized AgentEvent objects.
 */
export function normalizeAntigravityEvent(
  message: Record<string, unknown>,
  context: NormalizeContext,
): AgentEvent[] {
  const event = text(message.event);
  const makeBase = () => ({
    provider: "antigravity-cli" as const,
    projectId: context.projectId,
    sequence: context.nextSequence(),
    timestamp: new Date().toISOString(),
    threadId: context.threadId,
    runId: context.runId,
  });

  if (event === "init") {
    return [{ ...makeBase(), type: "provider.event", method: "init", payload: message }];
  }

  if (event === "step_update") {
    const step = record(message.step_update);
    const stepType = text(step.step_type);
    const textDelta = text(step.text_delta);
    const events: AgentEvent[] = [];

    if (textDelta && stepType === "agent_response") {
      events.push({ ...makeBase(), type: "assistant.delta", text: textDelta });
    } else if (textDelta && (stepType === "reasoning" || stepType === "thought")) {
      events.push({ ...makeBase(), type: "reasoning.delta", text: textDelta });
    }

    if (stepType === "tool") {
      const state = text(step.state)?.toUpperCase() ?? "";
      const itemId = String(step.step_index ?? "unknown");
      const tool = text(step.tool_name) ?? text(record(step.tool_info).name) ?? "tool";
      if (state === "RUNNING" || state === "PENDING") {
        events.push({ ...makeBase(), type: "tool.started", itemId, tool, summary: tool });
      } else if (state) {
        events.push({
          ...makeBase(),
          type: "tool.completed",
          itemId,
          tool,
          success: state === "DONE" || state === "SUCCESS" || state === "COMPLETED",
        });
      }
    }

    if (step.usage !== undefined) {
      events.push({ ...makeBase(), type: "usage.updated", usage: step.usage });
    }
    return events.length > 0
      ? events
      : [{ ...makeBase(), type: "provider.event", method: "step_update", payload: message }];
  }

  if (event === "result") {
    const result = record(message.result);
    const status = (text(result.status) ?? "ERROR").toUpperCase();
    const events: AgentEvent[] = [
      { ...makeBase(), type: "provider.event", method: "result", payload: message },
    ];
    if (result.usage !== undefined) {
      events.push({ ...makeBase(), type: "usage.updated", usage: result.usage });
    }
    if (status === "SUCCESS" || status === "DONE" || status === "COMPLETED") {
      events.push({ ...makeBase(), type: "run.completed", status: "completed" });
    } else if (status === "CANCELED" || status === "INTERRUPTED") {
      events.push({ ...makeBase(), type: "run.completed", status: "interrupted" });
    } else {
      events.push({
        ...makeBase(),
        type: "run.failed",
        message: text(result.error) ?? `Antigravity run ended with status ${status}`,
        code: status,
      });
    }
    return events;
  }

  return [{ ...makeBase(), type: "provider.event", method: event ?? "unknown", payload: message }];
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function record(value: unknown): Record<string, unknown> {
  return isRecord(value) ? value : {};
}

function text(value: unknown): string | null {
  return typeof value === "string" ? value : null;
}
