package agents

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/synthify/backend/apps/worker/pkg/worker/repository"
	"github.com/synthify/backend/apps/worker/pkg/worker/storage"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/builtin"
	toolsio "github.com/synthify/backend/apps/worker/pkg/worker/tools/builtin/io"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/builtin/memory"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core/base"
	"github.com/synthify/backend/apps/worker/pkg/worker/transform"
	jobstatus "github.com/synthify/backend/internal/platform/job/status"
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
	// htmlSummaryCount counts generate_html_summary completions within a job so
	// the per-item progress message ("N件目...") advances without rewinding the
	// fixed stage percent. Reset per job.
	htmlSummaryCount atomic.Int64
	// lastActivityWrite is the unix-nano timestamp of the last latestActivity
	// Firestore write, used to throttle activity updates (see writeActivity).
	lastActivityWrite atomic.Int64
	base              *base.Context
	repo              Repo
	fs                *storage.FileSystem
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
	"generate_html_summary":   "html_summary",
	"persist_knowledge_tree":  "persistence",
}

// perItemStageTools are stage tools invoked once *per item* within a job, so a
// single fixed stage name would make every call overwrite the same checkpoint.
// Their checkpoint key is suffixed with the item's local_id so each item's
// generated output is checkpointed independently and a resumed job can skip the
// ones already done (the expensive generate_html_summary pass).
var perItemStageTools = map[string]bool{
	"generate_html_summary": true,
}

// stageProgress maps a fixed stage name to the progress% to report on the
// stage's completion and a human-readable message. The values sit between
// Running (5%) and Completed (100%) and increase monotonically so the Firestore
// progress bar advances through the pipeline. Written fire-and-forget from the
// after-tool callback; missing stages just don't emit progress.
var stageProgress = map[string]struct {
	percent int
	message string
}{
	"briefing":       {25, "ドキュメントの概要を作成しました"},
	"knowledge_tree": {55, "ナレッジツリーを構築しています"},
	"html_summary":   {80, "各項目の要約を生成しています"},
	"persistence":    {95, "結果を保存しています"},
}

// toolActivity maps a tool name to a short human-readable line shown to the
// user as "what the worker is doing now". Tools not listed emit no activity.
// Kept deliberately vague (no document content) — these are status lines, not
// LLM output.
var toolActivity = map[string]string{
	"extract_text":            "ドキュメントを読み込んでいます",
	"semantic_chunking":       "ドキュメントを分割しています",
	"generate_brief":          "概要を作成しています",
	"analyze_dependencies":    "構成を分析しています",
	"semantic_search":         "関連箇所を検索しています",
	"grep_search":             "ドキュメント内を検索しています",
	"extract_table_data":      "表を解析しています",
	"repair_encoding":         "文字化けを修復しています",
	"glossary_register":       "用語を登録しています",
	"journal_add_task":        "作業計画を立てています",
	"journal_update_task":     "進捗を更新しています",
	"generate_knowledge_tree": "ナレッジツリーを構築しています",
	"generate_html_summary":   "項目の要約を生成しています",
	"quality_critique":        "品質をチェックしています",
	"deduplicate_and_merge":   "重複する概念を統合しています",
	"create_transform":        "データ変換を準備しています",
	"persist_knowledge_tree":  "結果を保存しています",
}

// activityThrottle bounds how often latestActivity is written to Firestore, so a
// fast burst of tool calls does not hammer the document.
const activityThrottle = 2 * time.Second

const currentCheckpointVersion = 1

// checkpointKey resolves the per-job checkpoint key for a tool call. For most
// stage tools it is just the fixed stage name; for per-item tools it appends
// the item's local_id so each item checkpoints separately. Returns "" when the
// tool is not a stage tool. The same function feeds both the read (resume) and
// write (record) paths so the keys can never diverge.
func checkpointKey(toolName string, args map[string]any) string {
	stage := stageTools[toolName]
	if stage == "" {
		return ""
	}
	if perItemStageTools[toolName] {
		if id := itemLocalID(args); id != "" {
			return stage + "/" + id
		}
		// No identifiable item — fall back to the bare stage so we at least do
		// not collide across tools, accepting that such calls share one slot.
	}
	return stage
}

// itemLocalID digs the item.local_id out of a generate_html_summary arg map.
// Args arrive as decoded JSON (map[string]any), so the item is a nested map.
func itemLocalID(args map[string]any) string {
	item, ok := args["item"].(map[string]any)
	if !ok {
		return ""
	}
	id, _ := item["local_id"].(string)
	return id
}

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
	o.htmlSummaryCount.Store(0)
	o.lastActivityWrite.Store(0)

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

// reportStageProgress writes the pipeline stage's progress to Firestore on stage
// completion. stage is the fixed stage name (per-item suffix already stripped).
// For the per-item html_summary stage the percent stays fixed and only the
// message advances ("N件目...") so the bar does not rewind. Fire-and-forget:
// notifier failures are non-fatal to the job.
func (o *Orchestrator) reportStageProgress(ctx context.Context, stage string) {
	if o.base == nil || o.base.Status == nil || o.base.Job == nil {
		return
	}
	sp, ok := stageProgress[stage]
	if !ok {
		return
	}
	message := sp.message
	if stage == "html_summary" {
		n := o.htmlSummaryCount.Add(1)
		message = fmt.Sprintf("%d件目の要約を生成しています", n)
	}
	fields := jobstatus.UpdateFields{
		CurrentStage: jobstatus.String(stage),
		Progress:     jobstatus.Int(sp.percent),
		Message:      jobstatus.String(message),
	}
	if err := o.base.Status.UpdateFields(ctx, o.base.Job.WorkspaceID, o.base.Job.JobID, fields); err != nil && o.base.Logger != nil {
		o.base.Logger.Warn("orchestrator.stage_progress_write_failed", "error", err.Error(), "stage", stage)
	}
}

// writeActivity publishes a per-tool human-readable activity line to Firestore,
// throttled to activityThrottle so a burst of tool calls does not hammer the
// document. toolName with no mapping is skipped. Fire-and-forget.
func (o *Orchestrator) writeActivity(ctx context.Context, toolName string) {
	if o.base == nil || o.base.Status == nil || o.base.Job == nil {
		return
	}
	msg, ok := toolActivity[toolName]
	if !ok {
		return
	}
	now := time.Now().UnixNano()
	last := o.lastActivityWrite.Load()
	if last != 0 && now-last < int64(activityThrottle) {
		return
	}
	// Best-effort CAS: if another goroutine won the slot, skip. (Tool callbacks
	// are sequential within a job, but guard anyway.)
	if !o.lastActivityWrite.CompareAndSwap(last, now) {
		return
	}
	fields := jobstatus.UpdateFields{LatestActivity: jobstatus.String(msg)}
	if err := o.base.Status.UpdateFields(ctx, o.base.Job.WorkspaceID, o.base.Job.JobID, fields); err != nil && o.base.Logger != nil {
		o.base.Logger.Warn("orchestrator.activity_write_failed", "error", err.Error(), "tool", toolName)
	}
}
