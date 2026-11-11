import { describe, expect, it } from "vitest";
import { normalizeAntigravityEvent, parseAntigravityLine } from "./normalize.js";

function context() {
  let sequence = 0;
  return { projectId: "project", threadId: "thread", runId: "run", nextSequence: () => ++sequence };
}

describe("normalizeAntigravityEvent", () => {
  it("normalizes streamed assistant text", () => {
    const [event] = normalizeAntigravityEvent({
      event: "step_update",
      step_update: { step_type: "agent_response", text_delta: "Hello" },
    }, context());
    expect(event).toMatchObject({ provider: "antigravity-cli", type: "assistant.delta", text: "Hello" });
  });

  it("normalizes tool completion", () => {
    const [event] = normalizeAntigravityEvent({
      event: "step_update",
      step_update: { step_index: 4, state: "DONE", step_type: "tool", tool_name: "run_command" },
    }, context());
    expect(event).toMatchObject({ type: "tool.completed", itemId: "4", tool: "run_command", success: true });
  });

  it("normalizes a successful result and usage", () => {
    const events = normalizeAntigravityEvent({
      event: "result",
      result: { status: "SUCCESS", usage: { total_tokens: 12 }, structured_output: { title: "Graph" } },
    }, context());
    expect(events.map((event) => event.type)).toEqual(["provider.event", "usage.updated", "run.completed"]);
    expect(events[0]).toMatchObject({ type: "provider.event", method: "result" });
  });
});

describe("parseAntigravityLine", () => {
  it("rejects non-object JSON", () => {
    expect(() => parseAntigravityLine("[]")).toThrow("non-object");
  });
});
