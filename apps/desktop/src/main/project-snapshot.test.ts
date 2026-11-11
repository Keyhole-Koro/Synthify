import { mkdir, mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import { buildFlowGraphExpansionPrompt, buildFlowGraphPrompt, buildProjectSnapshot } from "./project-snapshot.js";

const cleanup: string[] = [];

afterEach(async () => {
  await Promise.all(cleanup.splice(0).map((entry) => rm(entry, { recursive: true, force: true })));
});

describe("buildProjectSnapshot", () => {
  it("captures useful text while excluding secrets and dependencies", async () => {
    const root = await mkdtemp(path.join(tmpdir(), "insightify-snapshot-"));
    cleanup.push(root);
    await mkdir(path.join(root, "src"));
    await mkdir(path.join(root, "node_modules"));
    await writeFile(path.join(root, "README.md"), "# Example\nArchitecture overview");
    await writeFile(path.join(root, "src", "main.ts"), "export const main = true;");
    await writeFile(path.join(root, ".env"), "TOKEN=secret");
    await writeFile(path.join(root, "node_modules", "ignored.js"), "ignored");

    const snapshot = await buildProjectSnapshot(root);

    expect(snapshot.files).toContain("README.md");
    expect(snapshot.files).toContain("src/main.ts");
    expect(snapshot.files).not.toContain(".env");
    expect(snapshot.files).not.toContain("node_modules/ignored.js");
    expect(buildFlowGraphPrompt(snapshot)).toContain("untrusted data");
    expect(buildFlowGraphExpansionPrompt(snapshot, {
      title: "Graph",
      summary: "Graph summary",
      nodes: [{ id: "root", title: "Root", summary: "Root summary", kind: "room", parentId: null, evidence: [] }],
      edges: [],
    }, "root")).toContain("Add 4-7 useful direct child nodes");
  });
});
