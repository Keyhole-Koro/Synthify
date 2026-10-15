import { execFile, spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import { randomUUID } from "node:crypto";
import { EventEmitter } from "node:events";
import readline from "node:readline";
import { promisify } from "node:util";
import type { AgentEvent, ProviderInstallation } from "@insightify/agent-runtime";
import { antigravityCliCapabilities } from "@insightify/agent-runtime";
import { normalizeAntigravityEvent, parseAntigravityLine } from "./normalize.js";

const execFileAsync = promisify(execFile);

type AntigravityClientOptions = {
  /** Project identifier attached to every normalized event. */
  projectId: string;
  /** Default working directory used when a run does not override it. */
  cwd: string;
  /** Executable name or absolute path; defaults to `agy`. */
  executable?: string;
  /** Version discovered by the provider probe, if available. */
  version?: string | null;
};

type ActiveRun = {
  child: ChildProcessWithoutNullStreams;
  threadId: string;
  canceled: boolean;
  terminal: boolean;
};

type AgentEventBaseKey = "provider" | "projectId" | "sequence" | "timestamp" | "threadId" | "runId";
type WithoutAgentEventBase<T> = T extends AgentEvent ? Omit<T, AgentEventBaseKey> : never;
type AgentEventPayload = WithoutAgentEventBase<AgentEvent>;

/**
 * Handle referencing an active Antigravity CLI run.
 *
 * @property threadId Conversation identifier. The CLI adapter currently uses the run ID
 *   as the thread ID because each invocation starts a fresh process.
 * @property runId Unique identifier used to cancel and correlate the subprocess.
 */
export type AntigravityRunHandle = {
  threadId: string;
  runId: string;
};

/**
 * Options configuring an Antigravity CLI execution turn.
 *
 * @property cwd Optional per-run working directory. Falls back to the client directory.
 * @property jsonSchema Optional JSON Schema passed to the CLI for structured output.
 */
export type AntigravityRunOptions = {
  cwd?: string;
  jsonSchema?: unknown;
};

/**
 * Checks whether the Antigravity CLI (`agy`) binary is installed and executable on the host system.
 *
 * @param executable - Name or absolute path of the `agy` binary. The default is resolved
 *   through the host process PATH.
 * @returns ProviderInstallation status including detected version, capabilities, and a
 *   human-readable error when the probe cannot execute the binary.
 */
export async function probeAntigravity(executable = "agy"): Promise<ProviderInstallation> {
  try {
    const { stdout, stderr } = await execFileAsync(executable, ["--version"], { timeout: 5_000 });
    const output = `${stdout}\n${stderr}`.trim();
    return {
      provider: "antigravity-cli",
      installed: true,
      version: output || null,
      error: null,
      capabilities: antigravityCliCapabilities(),
    };
  } catch (error) {
    return {
      provider: "antigravity-cli",
      installed: false,
      version: null,
      error: error instanceof Error ? error.message : String(error),
      capabilities: antigravityCliCapabilities(),
    };
  }
}

/**
 * Client supervising child processes spawned via the Antigravity CLI (`agy`).
 */
export class AntigravityCliClient {
  readonly #projectId: string;
  readonly #cwd: string;
  readonly #executable: string;
  readonly #version: string | null;
  readonly #events = new EventEmitter();
  readonly #runs = new Map<string, ActiveRun>();
  #sequence = 0;

  /**
   * Constructs an AntigravityCliClient instance.
   *
   * @param options - Project identity, default working directory, executable, and optional
   *   probed version used to decorate emitted events.
   */
  constructor(options: AntigravityClientOptions) {
    this.#projectId = options.projectId;
    this.#cwd = options.cwd;
    this.#executable = options.executable ?? "agy";
    this.#version = options.version ?? null;
  }

  /**
   * Subscribes a listener to streaming AgentEvents emitted by this client.
   *
   * @param listener - Callback invoked synchronously for each normalized provider event.
   *   The callback should be short-lived; it runs on the child-process output path.
   * @returns Unsubscribe function that removes only the supplied listener and is safe to
   *   call more than once.
   */
  onEvent(listener: (event: AgentEvent) => void): () => void {
    this.#events.on("event", listener);
    return () => this.#events.off("event", listener);
  }

  /**
   * Launches a new `agy` run subprocess with the provided prompt and stream-json output format.
   *
   * @param prompt - Non-empty user instruction sent to the CLI.
   * @param options - Optional working-directory and structured-output overrides.
   * @returns AntigravityRunHandle with identifiers used to correlate events and cancel the run.
   * @throws Rejects if the child process cannot be spawned. Provider-level failures are
   *   reported as `run.failed` events after the process starts.
   */
  async startRun(prompt: string, options: AntigravityRunOptions = {}): Promise<AntigravityRunHandle> {
    const runId = randomUUID();
    const threadId = runId;
    const args = ["-p", prompt, "--output-format", "stream-json", "--sandbox"];
    if (options.jsonSchema !== undefined) args.push("--json-schema", JSON.stringify(options.jsonSchema));
    const child = spawn(
      this.#executable,
      args,
      { cwd: options.cwd ?? this.#cwd, stdio: ["pipe", "pipe", "pipe"], windowsHide: true },
    );
    const active: ActiveRun = { child, threadId, canceled: false, terminal: false };
    this.#runs.set(runId, active);

    readline.createInterface({ input: child.stdout }).on("line", (line) => {
      try {
        const events = normalizeAntigravityEvent(parseAntigravityLine(line), {
          projectId: this.#projectId,
          threadId,
          runId,
          nextSequence: () => ++this.#sequence,
        });
        for (const event of events) {
          if (event.type === "run.completed" || event.type === "run.failed") active.terminal = true;
          this.#emit(event);
        }
      } catch (error) {
        this.#warning(threadId, runId, error instanceof Error ? error.message : String(error));
      }
    });
    readline.createInterface({ input: child.stderr }).on("line", (line) => {
      if (line.trim()) this.#warning(threadId, runId, line.trim());
    });
    child.once("exit", (code, signal) => this.#handleExit(runId, code, signal));

    await new Promise<void>((resolve, reject) => {
      const onStartError = (error: Error) => {
        this.#runs.delete(runId);
        reject(error);
      };
      child.once("error", onStartError);
      child.once("spawn", () => {
        child.off("error", onStartError);
        this.#emitBase(threadId, runId, { type: "provider.connected", version: this.#version });
        this.#emitBase(threadId, runId, { type: "run.started" });
        resolve();
      });
    });
    child.once("error", (error) => {
      if (active.terminal) return;
      active.terminal = true;
      this.#emitBase(threadId, runId, { type: "run.failed", message: error.message, code: error.name });
    });
    child.stdin.end();
    return { threadId, runId };
  }

  /**
   * Cancels an active run by sending SIGINT to the `agy` process.
   * @param runId - Target run ID to cancel.
   */
  async cancelRun(runId: string): Promise<void> {
    const active = this.#runs.get(runId);
    if (!active) return;
    active.canceled = true;
    active.child.kill("SIGINT");
  }

  /**
   * Terminates all currently active `agy` child processes with SIGTERM.
   */
  async stop(): Promise<void> {
    for (const active of this.#runs.values()) {
      active.canceled = true;
      active.child.kill("SIGTERM");
    }
  }

  #handleExit(runId: string, code: number | null, signal: NodeJS.Signals | null): void {
    const active = this.#runs.get(runId);
    if (!active) return;
    this.#runs.delete(runId);
    if (!active.terminal) {
      if (active.canceled) {
        this.#emitBase(active.threadId, runId, { type: "run.completed", status: "interrupted" });
      } else if (code === 0) {
        this.#emitBase(active.threadId, runId, { type: "run.completed", status: "completed" });
      } else {
        this.#emitBase(active.threadId, runId, {
          type: "run.failed",
          message: `Antigravity CLI exited (${signal ?? code ?? "unknown"})`,
          code: code === null ? signal : String(code),
        });
      }
    }
    this.#emitBase(active.threadId, runId, {
      type: "provider.disconnected",
      reason: `Antigravity CLI exited (${signal ?? code ?? "unknown"})`,
    });
  }

  #warning(threadId: string, runId: string, message: string): void {
    this.#emitBase(threadId, runId, { type: "warning", message });
  }

  #emitBase(
    threadId: string,
    runId: string,
    event: AgentEventPayload,
  ): void {
    this.#emit({
      ...event,
      provider: "antigravity-cli",
      projectId: this.#projectId,
      sequence: ++this.#sequence,
      timestamp: new Date().toISOString(),
      threadId,
      runId,
    } as AgentEvent);
  }

  #emit(event: AgentEvent): void {
    this.#events.emit("event", event);
  }
}
