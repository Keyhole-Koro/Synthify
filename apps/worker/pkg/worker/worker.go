package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	connect "connectrpc.com/connect"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/synthify/backend/apps/worker/pkg/worker/agents"
	"github.com/synthify/backend/apps/worker/pkg/worker/config"
	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
	"github.com/synthify/backend/apps/worker/pkg/worker/llm"
	"github.com/synthify/backend/apps/worker/pkg/worker/metering"
	"github.com/synthify/backend/apps/worker/pkg/worker/repository"
	"github.com/synthify/backend/apps/worker/pkg/worker/storage"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core/base"
	"github.com/synthify/backend/apps/worker/pkg/worker/transform"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
	workerv1 "github.com/synthify/backend/internal/gen/synthify/worker/v1"
	workerv1connect "github.com/synthify/backend/internal/gen/synthify/worker/v1/workerv1connect"
	joblifecycle "github.com/synthify/backend/internal/platform/job/lifecycle"
	joblog "github.com/synthify/backend/internal/platform/job/log"
	jobstatus "github.com/synthify/backend/internal/platform/job/status"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/api/idtoken"
	"google.golang.org/genai"
)

var (
	ErrApprovalRequired = domain.ErrApprovalRequired
	ErrPlanRejected     = domain.ErrPlanRejected
)

type Repository interface {
	repository.AccountRepository
	repository.WorkspaceRepository
	repository.DocumentRepository
	repository.TreeRepository
	repository.ItemRepository
	repository.CheckpointRepository
	repository.DynamicToolRepository
}

type Worker struct {
	orchestrator *agents.Orchestrator
	repo         Repository
	lifecycle    joblifecycle.RuntimeController
	status       jobstatus.Notifier
	logger       *slog.Logger
	// agentBudget bounds the agent run's wall-clock so the worker aborts and
	// fails the job itself, ahead of Cloud Run's hard request-ctx cancel (which
	// risks a RUNNING stall). Zero means "no internal budget" (local/tests).
	agentBudget time.Duration
}

type ExecutePlanRequest = domain.ExecutePlanRequest

// dynamicSource adapts the Repository's DynamicToolRepository surface to the
// core.DynamicToolSource seam (the orchestrator only knows the seam).
type dynamicSource struct {
	repo repository.DynamicToolRepository
}

func (d dynamicSource) ResolveActive(ctx context.Context, workspaceID string) ([]*domain.DynamicTool, error) {
	return d.repo.ResolveActiveTools(ctx, workspaceID)
}

func NewWorkerWithNotifier(repo Repository, treeRepo Repository, notifier jobstatus.Notifier, m model.LLM, embedder base.Embedder, llmClient base.LLMClient, reporter llm.UsageReporter, fs *storage.FileSystem, logger *slog.Logger, nrApp *newrelic.Application, agentBudget time.Duration) (*Worker, error) {
	usage := base.NewUsageLimiter(treeRepo, logger)
	b := &base.Context{
		Repo:     treeRepo,
		Embedder: embedder,
		LLM:      base.NewCountingLLMClient(llmClient, usage),
		Usage:    usage,
		FS:       fs,
		Logger:   logger,
		Status:   notifier,
	}
	// dynamic tool wiring: repo doubles as the DB-backed source; the engine
	// matches the Starlark runtime that create_transform already uses (Phase 1
	// of dynamic-tool-synthesis). Python executor is a separate service in a
	// later phase.
	dynSrc := dynamicSource{repo: repo}
	dynEngine := transform.NewStarlarkEngine(10 * time.Second)
	// The agent path drives the model directly through ADK, bypassing the
	// metering.LLMClient wrapper, so usage is metered via an after-model
	// callback instead. See metering.NewAfterModelCallback.
	// LOG_LLM_PAYLOAD also turns on raw agent-response logging (text +
	// functionCall presence per turn), which the GeminiClient flag does not
	// cover because the agent path drives the model through ADK.
	logPayload := config.LoadLLM().LogPayload
	afterModelCBs := []llmagent.AfterModelCallback{
		metering.NewAfterModelCallback(reporter, logger),
		agents.NewPayloadLoggingCallback(logPayload, logger),
	}
	orch, err := agents.NewOrchestrator(m, b, repo, fs, dynSrc, dynEngine, afterModelCBs)
	if err != nil {
		return nil, err
	}

	return &Worker{
		orchestrator: orch,
		repo:         repo,
		lifecycle:    joblifecycle.New(repo, notifier, logger, nrApp),
		status:       notifier,
		logger:       logger,
		agentBudget:  agentBudget,
	}, nil
}

// shouldSkipJobStatus encodes the idempotency rules for at-least-once
// delivery via Cloud Tasks. A terminal job (succeeded / failed) must not
// be re-run: its outputs already exist and re-running risks producing
// duplicates. An already-running job is also skipped; the in-progress
// invocation will record its own completion, so a parallel second pass
// would race the first against the same document. Anything else — queued,
// unspecified, and RETRYABLE (a job the worker aborted on its own budget and
// deliberately requeued) — is allowed through so it runs / resumes.
func shouldSkipJobStatus(status appv1.JobLifecycleState) (bool, string) {
	switch status {
	case appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_SUCCEEDED,
		appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_FAILED:
		return true, "worker.skip_terminal_job"
	case appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_RUNNING:
		return true, "worker.skip_running_job"
	}
	return false, ""
}

func (w *Worker) Process(ctx context.Context, req ExecutePlanRequest) error {
	if err := req.Validate(); err != nil {
		return fmt.Errorf("invalid request: %w", err)
	}
	joblog.FromContext(ctx).Log(ctx, joblog.Event{
		JobID:       req.JobID,
		WorkspaceID: req.WorkspaceID,
		DocumentID:  req.DocumentID,
		Level:       joblog.INFO,
		Event:       "job.running",
		Message:     fmt.Sprintf("LLM worker processing job (doc: %s)", req.DocumentID),
		Detail:      map[string]any{"document_id": req.DocumentID},
	})

	job, err := w.repo.GetProcessingJob(ctx, req.JobID)
	if err != nil {
		return fmt.Errorf("get job %s: %w", req.JobID, err)
	}

	if skip, reason := shouldSkipJobStatus(job.Status); skip {
		w.logger.Info(reason,
			"job_id", req.JobID,
			"status", job.Status.String(),
		)
		return nil
	}

	if _, err := w.repo.GetDocument(ctx, req.DocumentID); err != nil {
		return fmt.Errorf("get document %s: %w", req.DocumentID, err)
	}

	// Tag the context with the account so the metering LLM wrapper can attribute
	// token usage to the right billing account. A missing workspace is treated
	// as non-fatal — the wrapper logs and drops usage rather than failing the job.
	if ws, wsErr := w.repo.GetWorkspace(ctx, req.WorkspaceID); wsErr == nil && ws != nil {
		ctx = metering.WithTag(ctx, metering.Tag{
			AccountID:   ws.AccountID,
			WorkspaceID: req.WorkspaceID,
			JobID:       req.JobID,
		})
	}

	payload := jobstatus.Payload{
		JobID:       req.JobID,
		DocumentID:  req.DocumentID,
		WorkspaceID: req.WorkspaceID,
		TreeID:      req.TreeID,
		JobType:     job.JobType.String(),
	}

	if approvals, err := w.repo.ListJobApprovalRequests(ctx, req.JobID); err == nil {
		for _, a := range approvals {
			if a.Status == string(domain.ActionStatusRejected) {
				return ErrPlanRejected
			}
		}
	}

	if err := w.lifecycle.MarkRunning(ctx, payload); err != nil {
		return fmt.Errorf("mark job running: %w", err)
	}

	// Bound the agent run by our own wall-clock budget (derived from the Cloud
	// Run request timeout, kept shorter) so we abort and fail the job ourselves
	// before Cloud Run hard-cancels the request ctx. A self-imposed deadline
	// here keeps the FAILED transition inside a still-live request, whereas a
	// Cloud Run hard cancel would also kill the failJob writes (the stall).
	agentCtx := ctx
	if w.agentBudget > 0 {
		var cancel context.CancelFunc
		agentCtx, cancel = context.WithTimeout(ctx, w.agentBudget)
		defer cancel()
	}
	if err := w.orchestrator.ProcessDocument(agentCtx, req.JobID, req.DocumentID, req.WorkspaceID, req.FileURI, req.Filename, req.MimeType); err != nil {
		// Distinguish a self-imposed budget abort (retryable) from a real
		// failure. A budget abort is: the agent ctx hit its deadline while the
		// parent request ctx is still alive. If the parent is also done, Cloud
		// Run hard-cancelled us — that is not safely retryable from here, so it
		// falls through to failJob (L4-a止血).
		if w.isBudgetAbort(ctx, agentCtx) && job.RetryCount < maxBudgetRetries {
			if rqErr := w.repo.RequeueProcessingJobForRetry(ctx, req.JobID); rqErr != nil {
				// If we cannot even mark it retryable, fail it so it does not
				// wedge in RUNNING.
				w.logger.Error("worker.requeue_failed", "error", rqErr.Error(), "job_id", req.JobID)
				w.failJob(ctx, req, payload, fmt.Errorf("%w: %w", domain.ErrAgentExecution, err))
				return err
			}
			w.logger.Info("worker.budget_abort_requeued",
				"job_id", req.JobID, "retry_count", job.RetryCount+1, "max", maxBudgetRetries)
			// Return the error so internal_dispatch responds 500 and Cloud Tasks
			// redelivers; the re-run resumes from checkpoints.
			return fmt.Errorf("agent budget exceeded, requeued for retry: %w", err)
		}
		// Wrap with ErrAgentExecution so classifyFailure can route this
		// path to agent_error via errors.Is. If the cause is itself a
		// Vertex APIError, errors.As still finds it through the wrap. failJob
		// runs on the parent ctx (still live), so the FAILED transition lands.
		wrapped := fmt.Errorf("%w: %w", domain.ErrAgentExecution, err)
		w.failJob(ctx, req, payload, wrapped)
		return err
	}

	joblog.FromContext(ctx).Log(ctx, joblog.Event{
		JobID:       req.JobID,
		WorkspaceID: req.WorkspaceID,
		DocumentID:  req.DocumentID,
		Level:       joblog.INFO,
		Event:       "job.completed",
		Message:     "LLM worker job completed successfully",
	})
	// A successful PROCESS_DOCUMENT job persisted nodes into the workspace's
	// single knowledge tree (creating new root nodes and/or merging into
	// existing ones). Signal TreeChanged so the frontend refetches the
	// workspace tree and diff-reveals the new nodes — there is no single
	// "document root" to graft anymore.
	outcome := joblifecycle.CompletionOutcome{
		TreeChanged: true,
	}
	if err := w.lifecycle.Complete(ctx, payload, outcome); err != nil {
		wrapped := fmt.Errorf("complete job in repo: %w", err)
		w.failJob(ctx, req, payload, wrapped)
		return wrapped
	}

	return nil
}

// maxBudgetRetries caps how many times a single job may be requeued after a
// wall-clock budget abort. Beyond this the job is failed for real, so a
// genuinely-too-slow document cannot loop forever. This is a worker-side backstop
// in addition to the Cloud Tasks queue's own max_attempts.
const maxBudgetRetries = 3

// isBudgetAbort reports whether the agent run ended because the worker's own
// wall-clock budget (agentCtx) expired while the request ctx is still live.
// That is the retryable case: we stopped ourselves, cleanly, before Cloud Run's
// hard cancel. If the parent ctx is also done, Cloud Run cancelled the request
// and we must not treat it as a clean self-abort.
func (w *Worker) isBudgetAbort(parent, agentCtx context.Context) bool {
	return w.agentBudget > 0 &&
		errors.Is(agentCtx.Err(), context.DeadlineExceeded) &&
		parent.Err() == nil
}

// failJobTimeout bounds the detached failure-transition work. A job that timed
// out had its request ctx cancelled by Cloud Run; the DB + Firestore writes that
// move it to FAILED must run on a ctx that is NOT already cancelled, or they fail
// too and the job wedges in RUNNING forever (the "Processing started" stall). We
// give those writes a fresh, short-lived ctx derived via WithoutCancel.
const failJobTimeout = 30 * time.Second

func (w *Worker) failJob(ctx context.Context, req ExecutePlanRequest, payload jobstatus.Payload, cause error) {
	reason := classifyFailure(cause)
	joblog.FromContext(ctx).Log(ctx, joblog.Event{
		JobID:       req.JobID,
		WorkspaceID: req.WorkspaceID,
		DocumentID:  req.DocumentID,
		Level:       joblog.ERROR,
		Event:       "job.failed",
		Message:     fmt.Sprintf("Agent execution failed: %v", cause),
		Detail:      map[string]any{"error": cause.Error(), "reason": string(reason)},
	})
	// Detach from the (possibly cancelled) request ctx so the FAILED transition
	// always lands. Keep the values (logging, tracing, metering tags) but drop
	// the cancellation, then bound it with our own timeout.
	failCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), failJobTimeout)
	defer cancel()
	w.lifecycle.TryFailWith(failCtx, payload, joblifecycle.Failure{Cause: cause, Reason: reason})
}

// classifyFailure maps a raw error from the agent / pipeline onto the
// Firestore reason enum the frontend switches on. Classification is done
// by error identity (errors.Is) or by structured type (errors.As) — never
// by message-string contains, so changing a wording upstream does not
// silently demote a known reason to "internal".
//
// New classifications belong here as additional cases. New source-side
// reasons are added by ensuring the producing code wraps with a recognisable
// sentinel or returns a typed error this function knows about.
func classifyFailure(err error) jobstatus.FirestoreJobStatusReason {
	if err == nil {
		return jobstatus.FirestoreJobStatusReasonInternal
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return jobstatus.FirestoreJobStatusReasonCancelled
	}
	if errors.Is(err, domain.ErrFileTooLarge) ||
		errors.Is(err, domain.ErrStorageQuotaExceeded) ||
		errors.Is(err, domain.ErrUploadSizeMismatch) {
		return jobstatus.FirestoreJobStatusReasonQuotaExceeded
	}
	// Vertex AI returns a structured APIError; "Publisher Model X was not
	// found" comes back as Code=404 + Status=NOT_FOUND. Switch on those
	// fields instead of grepping the message, so a reworded server reply
	// does not change the classification.
	var apiErr *genai.APIError
	if errors.As(err, &apiErr) {
		if apiErr.Code == http.StatusNotFound && apiErr.Status == "NOT_FOUND" {
			return jobstatus.FirestoreJobStatusReasonVertexModelNotFound
		}
		// Any other Vertex/Gemini API error counts as an agent error
		// because the agent owns the call site.
		return jobstatus.FirestoreJobStatusReasonAgentError
	}
	if errors.Is(err, domain.ErrAgentExecution) {
		return jobstatus.FirestoreJobStatusReasonAgentError
	}
	return jobstatus.FirestoreJobStatusReasonInternal
}

type Planner struct {
	repo   Repository
	llm    model.LLM
	logger *slog.Logger
}

func NewPlanner(repo Repository, llm model.LLM, logger *slog.Logger) *Planner {
	return &Planner{repo: repo, llm: llm, logger: logger}
}

func (p *Planner) GenerateExecutionPlan(ctx context.Context, req ExecutePlanRequest) (*domain.JobExecutionPlan, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if _, err := p.repo.GetDocument(ctx, req.DocumentID); err != nil {
		return nil, fmt.Errorf("get document %s: %w", req.DocumentID, err)
	}
	signals, err := p.repo.GetJobPlanningSignals(ctx, req.DocumentID, req.WorkspaceID, req.TreeID)
	if err != nil {
		p.logger.Error("planner.get_signals_failed", "error", err.Error(), "doc_id", req.DocumentID)
	}
	payload := map[string]any{
		"steps": []map[string]any{
			{"name": "text_extraction", "operation": appv1.JobOperation_JOB_OPERATION_READ_DOCUMENT.String(), "risk_tier": "tier_0"},
			{"name": "semantic_chunking", "operation": appv1.JobOperation_JOB_OPERATION_INVOKE_LLM.String(), "risk_tier": "tier_0"},
			{"name": "generate_knowledge_tree", "operation": appv1.JobOperation_JOB_OPERATION_CREATE_ITEM.String(), "risk_tier": "tier_1"},
			{"name": "persistence", "operation": appv1.JobOperation_JOB_OPERATION_CREATE_ITEM.String(), "risk_tier": "tier_1"},
			{"name": "evaluation", "operation": appv1.JobOperation_JOB_OPERATION_EMIT_EVAL.String(), "risk_tier": "tier_0"},
		},
		"document_id":  req.DocumentID,
		"workspace_id": req.WorkspaceID,
		"tree_id":      req.TreeID,
		"signals":      signals,
	}
	planJSONBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal execution plan: %w", err)
	}
	plan := &domain.JobExecutionPlan{
		PlanID:    "plan_" + req.JobID,
		JobID:     req.JobID,
		Status:    "approved",
		Summary:   "Extract text, chunk semantically, generate knowledge tree items, persist mutations, and evaluate the job.",
		PlanJSON:  string(planJSONBytes),
		CreatedBy: "llm_worker",
	}
	if err := p.repo.UpsertJobExecutionPlan(ctx, req.JobID, plan); err != nil {
		return nil, fmt.Errorf("failed to upsert execution plan: %w", err)
	}
	return plan, nil
}

type JobEvaluator struct {
	agent  *agents.Evaluator
	repo   Repository
	logger *slog.Logger
}

func NewJobEvaluator(repo Repository, llmClient llm.Client, logger *slog.Logger) *JobEvaluator {
	return &JobEvaluator{repo: repo, agent: agents.NewEvaluator(llmClient), logger: logger}
}

func (e *JobEvaluator) Evaluate(ctx context.Context, jobID string) (*domain.JobEvaluationResult, error) {
	job, err := e.repo.GetProcessingJob(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("get job %s: %w", jobID, err)
	}

	// The workspace is the tree root; evaluate against the full workspace
	// tree (all nodes, flat list with parent_id links).
	tree, err := e.repo.GetTreeByWorkspace(ctx, job.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("get workspace tree %s: %w", job.WorkspaceID, err)
	}

	treeJSON, err := json.Marshal(tree)
	if err != nil {
		return nil, fmt.Errorf("marshal tree: %w", err)
	}

	out, err := e.agent.EvaluateTree(ctx, string(treeJSON))
	if err != nil {
		return nil, err
	}

	findings := make([]string, len(out.Findings))
	copy(findings, out.Findings)

	result := &domain.JobEvaluationResult{
		JobID:    jobID,
		Passed:   out.Passed,
		Score:    int32(out.Score),
		Summary:  out.Summary,
		Findings: findings,
		Status:   "failed",
	}
	if out.Passed {
		result.Status = "passed"
	}

	e.repo.UpsertJobEvaluation(ctx, jobID, result)
	joblog.FromContext(ctx).Log(ctx, joblog.Event{
		JobID:       jobID,
		WorkspaceID: job.WorkspaceID,
		DocumentID:  job.DocumentID,
		Level:       joblog.INFO,
		Event:       "evaluation.completed",
		Message:     fmt.Sprintf("evaluation: passed=%v score=%d findings=%d", result.Passed, result.Score, len(result.Findings)),
		Detail: map[string]any{
			"passed":   result.Passed,
			"score":    result.Score,
			"findings": result.Findings,
		},
	})
	return result, nil
}

type HTTPDispatcher struct {
	baseURL string
}

func NewHTTPDispatcher(baseURL string) *HTTPDispatcher {
	return &HTTPDispatcher{baseURL: baseURL}
}

func (d *HTTPDispatcher) GenerateExecutionPlan(ctx context.Context, req ExecutePlanRequest) error {
	httpClient, err := d.httpClient(ctx)
	if err != nil {
		return fmt.Errorf("http client: %w", err)
	}
	client := workerv1connect.NewWorkerServiceClient(httpClient, strings.TrimRight(d.baseURL, "/"))
	rpcReq := connect.NewRequest(&workerv1.GenerateExecutionPlanRequest{
		JobId:       req.JobID,
		JobType:     req.JobType,
		DocumentId:  req.DocumentID,
		WorkspaceId: req.WorkspaceID,
		TreeId:      req.TreeID,
		Filename:    req.Filename,
		MimeType:    req.MimeType,
	})
	if _, err = client.GenerateExecutionPlan(ctx, rpcReq); err != nil {
		return fmt.Errorf("GenerateExecutionPlan rpc: %w", err)
	}
	return nil
}

func (d *HTTPDispatcher) ExecuteApprovedPlan(ctx context.Context, req ExecutePlanRequest) error {
	httpClient, err := d.httpClient(ctx)
	if err != nil {
		return fmt.Errorf("http client: %w", err)
	}
	client := workerv1connect.NewWorkerServiceClient(httpClient, strings.TrimRight(d.baseURL, "/"))
	rpcReq := connect.NewRequest(&workerv1.ExecuteApprovedPlanRequest{
		JobId:       req.JobID,
		JobType:     req.JobType,
		DocumentId:  req.DocumentID,
		WorkspaceId: req.WorkspaceID,
		TreeId:      req.TreeID,
		FileUri:     req.FileURI,
		Filename:    req.Filename,
		MimeType:    req.MimeType,
	})
	_, err = client.ExecuteApprovedPlan(ctx, rpcReq)
	if err != nil && connect.CodeOf(err) == connect.CodeFailedPrecondition {
		return ErrApprovalRequired
	}
	return err
}

func (d *HTTPDispatcher) httpClient(ctx context.Context) (*http.Client, error) {
	baseURL := strings.TrimRight(d.baseURL, "/")
	if !strings.HasPrefix(baseURL, "https://") {
		return http.DefaultClient, nil
	}
	return idtoken.NewClient(ctx, baseURL)
}
