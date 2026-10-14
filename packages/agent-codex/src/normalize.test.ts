import { describe, expect, it } from "vitest";
import { normalizeCodexNotification, normalizeCodexServerRequest } from "./normalize.js";

describe("normalizeCodexNotification", () => {
  it("normalizes streamed assistant text", () => {
    const [event] = normalizeCodexNotification(
      {
        method: "item/agentMessage/delta",
        params: { threadId: "thr-1", turnId: "turn-1", delta: "hello" },
      },
      { projectId: "project-1", sequence: 7 },
    );

    expect(event).toMatchObject({
      type: "assistant.delta",
      projectId: "project-1",
      threadId: "thr-1",
      runId: "turn-1",
      sequence: 7,
      text: "hello",
    });
  });

  it("normalizes failed turns", () => {
    const [event] = normalizeCodexNotification(
      {
        method: "turn/completed",
        params: {
          threadId: "thr-1",
          turn: {
            id: "turn-1",
            status: "failed",
            error: { message: "limit", codexErrorInfo: { type: "UsageLimitExceeded" } },
          },
        },
      },
      { projectId: "project-1", sequence: 8 },
    );

    expect(event).toMatchObject({
      type: "run.failed",
      message: "limit",
      code: "UsageLimitExceeded",
    });
  });
});

describe("normalizeCodexServerRequest", () => {
  it("normalizes command approvals without executing the command", () => {
    const [event] = normalizeCodexServerRequest(
      {
        method: "item/commandExecution/requestApproval",
        id: 42,
        params: {
          threadId: "thr-1",
          turnId: "turn-1",
          itemId: "item-1",
          command: ["bun", "test"],
          cwd: "/repo",
          reason: "Run the tests",
        },
      },
      { projectId: "project-1", sequence: 9 },
    );

    expect(event).toMatchObject({
      type: "approval.requested",
      requestId: "42",
      approvalKind: "command",
      command: "bun test",
      cwd: "/repo",
    });
  });
});
