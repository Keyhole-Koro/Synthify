package agents

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/synthify/backend/apps/worker/pkg/worker/repository"
	"github.com/synthify/backend/apps/worker/pkg/worker/storage"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/builtin"
	toolsio "github.com/synthify/backend/apps/worker/pkg/worker/tools/builtin/io"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/builtin/memory"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core/base"
	"github.com/synthify/backend/apps/worker/pkg/worker/transform"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

// Orchestrator owns the agent wiring for the worker. The agent is rebuilt per
// job so dynamic tools (DB-resolved, workspace-scoped) can merge with the
// static builtin set without leaking across jobs. For builtin-only runs (no
// DynamicSource, or empty workspace) the agent's Tools set is identical to
// what the pre-道A wiring produced.
// Repo is the persistence surface the orchestrator needs: checkpoint state for
// resume/save and per-tool-call logging. Worker passes its full Repository
// composite, which already satisfies both.
type Repo interface {
	repository.CheckpointRepository
	LogToolCall(ctx context.Context, jobID, toolName, inputJSON, outputJSON string, durationMs int64) error
}

type Orchestrator struct {
	currentJobID atomic.Pointer[string]
	// dynamicToolNames is the set of dynamic (transform-backed) tool names for
	// the current job, used by beforeToolCallbacks to count transform runs
	// separately from builtin tool runs. Rebuilt per job alongside currentJobID.
	dynamicToolNames atomic.Pointer[map[string]struct{}]
	// repeatGuard tracks consecutive identical (tool, args) calls within a job so
	// a stuck agent that keeps re-issuing the same tool call is nudged off the
	// loop before it burns the whole request budget. Rebuilt per job.
	repeatGuard atomic.Pointer[repeatTracker]
	base        *base.Context
	repo        Repo
	fs          *storage.FileSystem
	// documentMap is the Working Memory block the deterministic extract+chunk
	// pre-pass populates at the start of each job (see ProcessDocument).
	documentMap *memory.DocumentMap

	// model and the inputs to per-job llmagent.New rebuilt by buildAgent.
	model          model.LLM
	instruction    string
	builtinTools   []core.Tool
	beforeModelCBs []llmagent.BeforeModelCallback
	afterModelCBs  []llmagent.AfterModelCallback
	beforeToolCBs  []llmagent.BeforeToolCallback
	afterToolCBs   []llmagent.AfterToolCallback
	dynamicSource  core.DynamicToolSource // optional; nil = builtin-only
	dynamicEngine  transform.Engine       // engine for executing dynamic tool Code
}

var stageTools = map[string]string{
	"generate_brief":          "briefing",
	"generate_knowledge_tree": "knowledge_tree",
	"persist_knowledge_tree":  "persistence",
}

const currentCheckpointVersion = 1

// NewOrchestrator wires the worker agent. dynSrc and dynEngine are optional
// (nil means "builtin tools only"). When non-nil, ProcessDocument resolves
// active dynamic tools for the job's workspace and merges them into the
// agent's tool set before running.
// afterModelCBs are appended to the agent's after-model callback chain (e.g.
// billing usage metering). nil is fine — builtin-only runs pass no extras.
func NewOrchestrator(m model.LLM, b *base.Context, repo Repo, fs *storage.FileSystem, dynSrc core.DynamicToolSource, dynEngine transform.Engine, afterModelCBs []llmagent.AfterModelCallback) (*Orchestrator, error) {
	builtins, err := builtin.Build(b)
	if err != nil {
		return nil, err
	}
	b.Memories = builtins.Memories

	orch := &Orchestrator{
		base:          b,
		repo:          repo,
		fs:            fs,
		documentMap:   builtins.DocumentMap,
		model:         m,
		instruction:   orchestratorInstruction,
		builtinTools:  builtins.Tools,
		dynamicSource: dynSrc,
		dynamicEngine: dynEngine,
		afterModelCBs: afterModelCBs,
	}

	orch.beforeModelCBs = orch.beforeModelCallbacks()
	orch.beforeToolCBs = orch.beforeToolCallbacks()
	orch.afterToolCBs = orch.afterToolCallbacks()

	return orch, nil
}

func (o *Orchestrator) ProcessDocument(ctx context.Context, jobID, documentID, workspaceID, fileURI, filename, mimeType string) error {
	if o.base != nil {
		o.base.BeginJob(ctx, jobID, workspaceID, documentID)
	}
	o.currentJobID.Store(&jobID)
	o.repeatGuard.Store(newRepeatTracker())

	// Deterministic extract + chunk pre-pass. Reading the source is not a
	// judgement call — file_uri is already known — so we do it in code rather
	// than letting the agent discover it by guessing extract_text args (the
	// loop that burned ~3 min and tripped the request timeout). The resulting
	// chunk map is auto-injected as Working Memory, matching the
	// "reads are injected, not tools" contract. Best-effort: on failure we log
	// and let the agent fall back to calling extract_text itself.
	o.prepareDocumentMap(ctx, documentID, fileURI, mimeType)

	// Per-job agent: builtin + workspace-resolved dynamic tools merged.
	dyn := o.resolveDynamicTools(ctx, workspaceID)
	dynNames := make(map[string]struct{}, len(dyn))
	for _, t := range dyn {
		dynNames[t.Name] = struct{}{}
	}
	o.dynamicToolNames.Store(&dynNames)
	jobAgent, err := o.buildAgent(dyn)
	if err != nil {
		return fmt.Errorf("build per-job agent: %w", err)
	}
	jobRunner, err := runner.New(runner.Config{
		AppName:           "synthify-worker",
		Agent:             jobAgent,
		SessionService:    session.InMemoryService(),
		AutoCreateSession: true,
	})
	if err != nil {
		return fmt.Errorf("build per-job runner: %w", err)
	}

	msg := fmt.Sprintf(
		"Process this document and build a knowledge tree.\n\njob_id: %s\ndocument_id: %s\nworkspace_id: %s\nfile_uri: %s\nfilename: %s\nmime_type: %s\n\nThe source has already been extracted and chunked for you — see the Document Map in Working Memory for the chunk_ids and outline. Do NOT call extract_text or re-chunk unless the Document Map is empty. Start from: generate brief, generate tree items, critique, then persist.",
		jobID, documentID, workspaceID, fileURI, filename, mimeType,
	)
	for event, err := range jobRunner.Run(ctx, "worker", jobID, genai.NewContentFromText(msg, genai.RoleUser), agent.RunConfig{}) {
		if err != nil {
			return fmt.Errorf("agent run: %w", err)
		}
		if event != nil && event.IsFinalResponse() {
			return nil
		}
	}
	return nil
}

// prepareDocumentMap extracts and chunks the source up front, persists the
// chunks, and populates the Document Map Working Memory block so the agent
// starts with the outline already in context. Best-effort by design: any
// failure here (extraction error, no map handle) is logged and the agent falls
// back to calling extract_text / semantic_chunking itself, so this pre-pass can
// only help, never block. Empty extracted text (e.g. an image with no OCR text)
// leaves the map empty and the agent proceeds via its own tools.
func (o *Orchestrator) prepareDocumentMap(ctx context.Context, documentID, fileURI, mimeType string) {
	if o.documentMap == nil || o.base == nil || o.base.Job == nil {
		return
	}
	logger := o.base.Logger
	rawText, err := toolsio.Extract(ctx, o.base, fileURI, mimeType, o.base.Job.WorkspaceID, documentID)
	if err != nil {
		if logger != nil {
			logger.Warn("orchestrator.document_map_extract_failed", "error", err.Error(), "document_id", documentID)
		}
		return
	}
	_, _, chunks, err := toolsio.ChunkAndSave(ctx, o.base, documentID, "", rawText)
	if err != nil {
		if logger != nil {
			logger.Warn("orchestrator.document_map_chunk_failed", "error", err.Error(), "document_id", documentID)
		}
		return
	}
	o.documentMap.Set(chunks)
	if logger != nil {
		logger.Info("orchestrator.document_map_ready", "document_id", documentID, "chunks", len(chunks))
	}
}
