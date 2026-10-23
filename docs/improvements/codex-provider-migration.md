# Codex provider migration

## Status

- **Priority**: P3 / architecture proposal
- **Decision**: Gemini remains the production default. Codex is introduced behind a provider boundary and validated first as a personal/local PoC.
- **Target**: replace the process-tool LLM client first, then decide whether to replace the Google ADK agent runtime.

## Context

Synthify currently uses Gemini in two distinct paths:

1. `apps/worker/pkg/worker/llm/GeminiClient`
   - text generation
   - structured JSON generation
   - source-file upload through the Gemini Files API
   - token accounting and job logging
2. Google ADK agent runtime
   - Gemini-backed `model.LLM`
   - orchestration and tool calling through `llmagent`
   - built-in and dynamic tools adapted to Google ADK tools

These paths have different migration costs. Replacing `GeminiClient` is a bounded provider change because process tools already depend on the small `llm.Client` interface. Replacing the ADK runtime requires an orchestration migration, tool transport changes, callback replacement, and new usage accounting.

## Product boundary

Codex authentication is not Synthify authentication.

- Firebase remains the source of truth for Synthify users, workspaces, authorization, and account ownership.
- ChatGPT/Codex authentication is an optional provider connection attached to an already authenticated Synthify user.
- A ChatGPT Pro session must never be shared across Synthify users.
- Provider credentials and Codex state must never be exposed to the browser.

The supported product shapes are:

| Product shape | Provider model | Decision |
| --- | --- | --- |
| Synthify Personal / local | Codex App Server with the user's ChatGPT session | Supported PoC target |
| Internal single-user deployment | Codex App Server with isolated user state | Possible after security review |
| Public multi-user Synthify Cloud | OpenAI API or Gemini/Vertex managed by Synthify | Production target |
| Shared operator ChatGPT Pro account for all users | Shared Codex session | Prohibited design |

## Goals

- Add a provider-neutral initialization seam without changing the current production default.
- Implement `GenerateText` and `GenerateStructured` through Codex App Server in a local-only PoC.
- Preserve the existing process-tool interface and usage metadata shape.
- Keep Firebase authentication and account provisioning unchanged.
- Isolate Codex state, workspace files, and subprocesses per user/job.
- Gather output-quality, latency, failure-mode, and quota-consumption data before replacing ADK.

## Non-goals

- Replacing Firebase authentication with ChatGPT authentication.
- Routing public browser traffic directly to Codex App Server.
- Sharing a single ChatGPT Pro account across multiple Synthify users.
- Replacing the Google ADK agent runtime in the first implementation.
- Treating ChatGPT subscription quota as a billable token cost in the existing Stripe usage pipeline.

## Proposed architecture

### Phase 1: process-tool provider seam

Introduce a provider selection at worker startup while retaining Gemini as the default.

```text
process tools
    |
    v
llm.Client
    |-- GeminiClient        (production default)
    `-- CodexClient         (local PoC only)
```

Suggested configuration:

```text
LLM_PROVIDER=gemini|codex
CODEX_APP_SERVER_COMMAND=codex app-server
CODEX_HOME=/isolated/path/per-user
CODEX_MODEL=
CODEX_SANDBOX=read-only
```

`LLM_PROVIDER=codex` must fail closed unless the runtime is explicitly marked as local/personal. Production and staging must continue to require a managed provider until the deployment model is reviewed.

### Phase 2: Codex App Server client

Add a worker-side client responsible for:

- spawning or connecting to Codex App Server
- JSON-RPC request/response correlation
- thread and turn lifecycle
- event streaming and cancellation
- structured output validation
- provider error normalization
- subprocess shutdown and cleanup
- user/job-specific `CODEX_HOME`

The browser must communicate only with Synthify API. The API/worker owns the Codex connection and credential boundary.

### Phase 3: source files

The current Gemini client uploads files to the Gemini Files API. Codex should instead receive a job-scoped local workspace:

```text
GCS source
  -> job-scoped temporary directory
  -> normalized/extracted files
  -> Codex working directory
  -> structured response
  -> secure cleanup
```

Requirements:

- one directory per job
- no cross-user or cross-workspace reuse
- read-only sandbox for the first PoC
- allowlisted root paths
- deterministic cleanup after success, failure, or cancellation
- explicit size and file-count limits

### Phase 4: authentication connection

Add a separate provider-connection state after Firebase login:

```text
Firebase session
  -> Synthify account and workspace authorization

Codex connection
  -> provider availability for this user/device
```

For a local/personal client, use the Codex App Server login flow. Store only the minimum connection metadata in Synthify. Tokens and Codex auth files remain in the isolated local runtime and must not be copied to Firestore, PostgreSQL, logs, or browser storage.

### Phase 5: evaluation gate

Before changing the production default, compare Gemini and Codex on representative fixtures:

- structured JSON validity
- knowledge-tree quality
- citation/source grounding
- tool selection accuracy
- latency and timeout behavior
- retry behavior
- large-document handling
- quota/rate-limit behavior
- deterministic cleanup

Use the existing eval runner where possible. Record provider, model, fixture, duration, input/output size, validation result, and failure class.

### Phase 6: agent-runtime decision

After the process-tool PoC, choose one of two paths:

1. **Keep Google ADK for orchestration** and use Codex only for process-tool calls.
2. **Replace Google ADK** with Codex thread/turn orchestration and expose Synthify tools through MCP or an equivalent stable tool boundary.

The second path is a separate migration. It must replace:

- `model.LLM` initialization
- `llmagent` orchestration
- ADK tool adapters
- before/after model callbacks
- before/after tool callbacks
- agent metering callbacks
- cancellation and checkpoint behavior

## Usage and billing

The existing billing path assumes provider-reported input/output tokens and attributes cost to the requesting Synthify account. A ChatGPT subscription-backed Codex session does not map cleanly to that model.

For the personal PoC:

- record usage as operational telemetry, not a monetary charge
- expose provider and quota/rate-limit state separately from Stripe cost
- do not add Codex subscription usage to customer invoices

For Synthify Cloud:

- use a managed API provider with explicit server-side metering
- retain current account attribution and budget enforcement semantics

## Security requirements

- Never expose Codex App Server directly to an untrusted network.
- Never send Codex credentials or auth files to the browser.
- Run one isolated Codex state directory per user/device identity.
- Run one isolated working directory per job.
- Default to read-only sandboxing.
- Require explicit approval before enabling write or shell-capable workflows.
- Redact prompts, outputs, file paths, and auth metadata from logs unless local debugging is explicitly enabled.
- Add process limits, request deadlines, cancellation, and orphan-process cleanup.
- Prevent a Firebase user from selecting another user's Codex connection.

## First implementation PR

The first code PR should be intentionally small:

1. add `LLM_PROVIDER` parsing with `gemini` as the default
2. split provider initialization from ADK initialization
3. introduce a `CodexClient` implementing `llm.Client`
4. support text and structured generation only
5. reject source files initially with a clear unsupported error, or stage them into a job directory if the file boundary is ready
6. add unit tests using a fake JSON-RPC transport
7. add a local-only integration test guarded by an environment variable
8. keep all production Terraform and defaults on Gemini

## Acceptance criteria for the PoC

- Existing Gemini tests and production startup behavior remain unchanged.
- `LLM_PROVIDER=codex` can run one text-generation fixture locally.
- Structured generation returns schema-valid JSON for the selected fixture.
- A cancelled request terminates the corresponding turn and subprocess work.
- No credentials or source contents appear in normal logs.
- Concurrent jobs cannot read each other's working directories or Codex state.
- Provider errors are surfaced as typed worker errors rather than raw JSON-RPC payloads.
- Usage telemetry distinguishes `gemini` from `codex` without generating Stripe charges for subscription-backed runs.

## Rollout

1. Merge provider-boundary refactor with no behavior change.
2. Land Codex local PoC behind `LLM_PROVIDER=codex`.
3. Run eval fixtures and document results.
4. Decide whether Codex remains a personal feature, becomes an API-backed cloud provider, or replaces ADK.
5. Only then update deployment configuration and product UI.

## References

- Codex App Server: <https://developers.openai.com/codex/app-server>
- Codex authentication: <https://developers.openai.com/codex/auth>
- Codex SDK: <https://developers.openai.com/codex/codex-sdk>
