import { randomUUID } from "node:crypto";
import { realpathSync } from "node:fs";
import path from "node:path";
import { DatabaseSync } from "node:sqlite";
import type { ProjectSummary } from "@insightify/desktop-bridge";
import { graphLayoutSchema, parseFlowGraph, type GeneratedFlowGraph } from "@insightify/graph-domain";

type ProjectRow = {
  id: string;
  display_name: string;
  canonical_path: string;
  last_opened_at: string;
};

type ProjectGraphRow = {
  project_id: string;
  provider: string;
  snapshot_hash: string;
  generated_at: string;
  graph_json: string;
  layout_json: string;
};

/**
 * Project metadata paired with its canonical filesystem path.
 *
 * @property canonicalPath Absolute, symlink-resolved path used for project identity.
 * @property id UUID inherited from ProjectSummary.
 * @property displayName User-facing project name inherited from ProjectSummary.
 * @property lastOpenedAt ISO-8601 timestamp inherited from ProjectSummary.
 */
export type ProjectMount = ProjectSummary & {
  canonicalPath: string;
};

/**
 * Storage repository managing registered projects and their associated generated flow graphs.
 */
export interface ProjectRepository {
  /** Opens or registers a project path, recording its lastOpenedAt timestamp. */
  openPath(selectedPath: string): ProjectMount;
  /** Lists all registered projects ordered by most recently opened. */
  list(): ProjectSummary[];
  /** Resolves project details by project UUID. */
  resolve(projectId: string): ProjectMount | null;
  /** Retrieves the persisted GeneratedFlowGraph for a project. */
  getGraph(projectId: string): GeneratedFlowGraph | null;
  /** Saves or updates a project's flow graph in the database. */
  saveGraph(value: GeneratedFlowGraph): void;
  /** Closes the underlying database connection. */
  close(): void;
}

/**
 * SQLite-backed implementation of ProjectRepository utilizing Node.js built-in DatabaseSync.
 */
export class SqliteProjectRepository implements ProjectRepository {
  readonly #database: DatabaseSync;

  /**
   * Constructs a SqliteProjectRepository and executes database schema migrations.
   * @param databasePath - Filesystem path to the SQLite database file.
   */
  constructor(databasePath: string) {
    this.#database = new DatabaseSync(databasePath);
    this.#database.exec("PRAGMA journal_mode = WAL; PRAGMA foreign_keys = ON;");
    this.#migrate();
  }

  openPath(selectedPath: string): ProjectMount {
    const canonicalPath = realpathSync.native(selectedPath);
    const now = new Date().toISOString();
    const current = this.#database
      .prepare("SELECT id FROM projects WHERE canonical_path = ?")
      .get(canonicalPath) as { id: string } | undefined;
    const id = current?.id ?? randomUUID();
    const displayName = path.basename(canonicalPath) || canonicalPath;

    this.#database
      .prepare(`
        INSERT INTO projects (id, display_name, canonical_path, last_opened_at)
        VALUES (?, ?, ?, ?)
        ON CONFLICT(canonical_path) DO UPDATE SET
          display_name = excluded.display_name,
          last_opened_at = excluded.last_opened_at
      `)
      .run(id, displayName, canonicalPath, now);

    return { id, displayName, canonicalPath, lastOpenedAt: now };
  }

  list(): ProjectSummary[] {
    const rows = this.#database
      .prepare(`
        SELECT id, display_name, canonical_path, last_opened_at
        FROM projects
        ORDER BY last_opened_at DESC
      `)
      .all() as ProjectRow[];
    return rows.map(toSummary);
  }

  resolve(projectId: string): ProjectMount | null {
    const row = this.#database
      .prepare(`
        SELECT id, display_name, canonical_path, last_opened_at
        FROM projects
        WHERE id = ?
      `)
      .get(projectId) as ProjectRow | undefined;
    return row ? { ...toSummary(row), canonicalPath: row.canonical_path } : null;
  }

  getGraph(projectId: string): GeneratedFlowGraph | null {
    const row = this.#database
      .prepare(`
        SELECT project_id, provider, snapshot_hash, generated_at, graph_json, layout_json
        FROM project_graphs
        WHERE project_id = ?
      `)
      .get(projectId) as ProjectGraphRow | undefined;
    if (!row) return null;
    if (row.provider !== "codex" && row.provider !== "antigravity-cli") {
      throw new Error(`Unsupported persisted graph provider: ${row.provider}`);
    }
    return {
      projectId: row.project_id,
      provider: row.provider,
      snapshotHash: row.snapshot_hash,
      generatedAt: row.generated_at,
      graph: parseFlowGraph(JSON.parse(row.graph_json)),
      layout: graphLayoutSchema.parse(JSON.parse(row.layout_json)),
    };
  }

  saveGraph(value: GeneratedFlowGraph): void {
    this.#database
      .prepare(`
        INSERT INTO project_graphs (project_id, provider, snapshot_hash, generated_at, graph_json, layout_json)
        VALUES (?, ?, ?, ?, ?, ?)
        ON CONFLICT(project_id) DO UPDATE SET
          provider = excluded.provider,
          snapshot_hash = excluded.snapshot_hash,
          generated_at = excluded.generated_at,
          graph_json = excluded.graph_json,
          layout_json = excluded.layout_json
      `)
      .run(value.projectId, value.provider, value.snapshotHash, value.generatedAt, JSON.stringify(value.graph), JSON.stringify(value.layout));
  }

  close(): void {
    this.#database.close();
  }

  #migrate(): void {
    this.#database.exec(`
      CREATE TABLE IF NOT EXISTS schema_migrations (
        version INTEGER PRIMARY KEY,
        applied_at TEXT NOT NULL
      );

      CREATE TABLE IF NOT EXISTS projects (
        id TEXT PRIMARY KEY,
        display_name TEXT NOT NULL,
        canonical_path TEXT NOT NULL UNIQUE,
        last_opened_at TEXT NOT NULL
      );

      CREATE TABLE IF NOT EXISTS project_graphs (
        project_id TEXT PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
        provider TEXT NOT NULL,
        snapshot_hash TEXT NOT NULL,
        generated_at TEXT NOT NULL,
        graph_json TEXT NOT NULL,
        layout_json TEXT NOT NULL DEFAULT '{}'
      );

      INSERT OR IGNORE INTO schema_migrations (version, applied_at)
      VALUES (1, CURRENT_TIMESTAMP);

      INSERT OR IGNORE INTO schema_migrations (version, applied_at)
      VALUES (2, CURRENT_TIMESTAMP);
    `);
    const graphColumns = this.#database.prepare("PRAGMA table_info(project_graphs)").all() as Array<{ name: string }>;
    if (!graphColumns.some((column) => column.name === "layout_json")) {
      this.#database.exec("ALTER TABLE project_graphs ADD COLUMN layout_json TEXT NOT NULL DEFAULT '{}'");
    }
    this.#database.exec(`
      INSERT OR IGNORE INTO schema_migrations (version, applied_at)
      VALUES (3, CURRENT_TIMESTAMP);
    `);
  }
}

function toSummary(row: ProjectRow): ProjectSummary {
  return {
    id: row.id,
    displayName: row.display_name,
    lastOpenedAt: row.last_opened_at,
  };
}
