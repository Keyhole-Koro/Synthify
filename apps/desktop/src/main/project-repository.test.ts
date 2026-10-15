import { realpathSync } from "node:fs";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import { SqliteProjectRepository } from "./project-repository.js";

const cleanup: string[] = [];

afterEach(async () => {
  await Promise.all(cleanup.splice(0).map((entry) => rm(entry, { recursive: true, force: true })));
});

describe("SqliteProjectRepository", () => {
  it("persists an opaque project id and never exposes its path in summaries", async () => {
    const directory = await mkdtemp(path.join(tmpdir(), "insightify-project-"));
    cleanup.push(directory);
    const repository = new SqliteProjectRepository(":memory:");

    const mount = repository.openPath(directory);
    const [summary] = repository.list();

    expect(mount.canonicalPath).toBe(realpathSync.native(directory));
    expect(summary).toEqual({
      id: mount.id,
      displayName: path.basename(directory),
      lastOpenedAt: mount.lastOpenedAt,
    });
    expect(summary).not.toHaveProperty("canonicalPath");

    repository.saveGraph({
      projectId: mount.id,
      provider: "codex",
      snapshotHash: "abc123",
      generatedAt: "2026-08-29T00:00:00.000Z",
      graph: {
        title: "Test flow",
        summary: "A stored graph",
        nodes: [{ id: "root", title: "Root", summary: "Root room", kind: "room", parentId: null, evidence: [] }],
        edges: [],
      },
      layout: { root: { x: 50, y: 48 } },
    });
    expect(repository.getGraph(mount.id)?.graph.title).toBe("Test flow");
    expect(repository.getGraph(mount.id)?.layout.root).toEqual({ x: 50, y: 48 });
    repository.close();
  });
});
