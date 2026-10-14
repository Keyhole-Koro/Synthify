// Package worker provides background job execution, planning, and evaluation engines.
package worker

import (
	"context"
	"errors"
	"log/slog"

	connect "connectrpc.com/connect"
	"github.com/synthify/backend/internal/domain"
	workerv1 "github.com/synthify/backend/internal/gen/synthify/worker/v1"
)

// ConnectHandler exposes Connect-RPC endpoints for worker execution planning and execution.
type ConnectHandler struct {
	processor interface {
		Process(ctx context.Context, req ExecutePlanRequest) error
	}
	jobRepo   Repository
	planner   *Planner
	evaluator *JobEvaluator
	logger    *slog.Logger
}

// NewConnectHandler constructs a new ConnectHandler for worker RPC requests.
func NewConnectHandler(processor interface {
	Process(ctx context.Context, req ExecutePlanRequest) error
}, jobRepo Repository, planner *Planner, evaluator *JobEvaluator, logger *slog.Logger) *ConnectHandler {
	return &ConnectHandler{
		processor: processor,
		jobRepo:   jobRepo,
		planner:   planner,
		evaluator: evaluator,
		logger:    logger,
	}
}

// GenerateExecutionPlan generates a multi-stage execution plan for a document processing job.
func (h *ConnectHandler) GenerateExecutionPlan(ctx context.Context, req *connect.Request[workerv1.GenerateExecutionPlanRequest]) (*connect.Response[workerv1.GenerateExecutionPlanResponse], error) {
	if req.Msg.GetJobId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("job_id is required"))
	}
	dispatchReq := ExecutePlanRequest{
		JobID:       req.Msg.GetJobId(),
		JobType:     req.Msg.GetJobType(),
		DocumentID:  req.Msg.GetDocumentId(),
		WorkspaceID: req.Msg.GetWorkspaceId(),
		TreeID:      req.Msg.GetTreeId(),
		Filename:    req.Msg.GetFilename(),
		MimeType:    req.Msg.GetMimeType(),
	}
	if err := dispatchReq.Validate(); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	h.logger.Info("worker.generate_plan_received", "job_id", req.Msg.GetJobId(), "doc_id", req.Msg.GetDocumentId())
	plan, err := h.planner.GenerateExecutionPlan(ctx, dispatchReq)
	if err != nil {
		return nil, workerConnectError(err)
	}
	return connect.NewResponse(&workerv1.GenerateExecutionPlanResponse{
		PlanId:   plan.PlanID,
		JobId:    plan.JobID,
		Status:   plan.Status,
		Summary:  plan.Summary,
		PlanJson: plan.PlanJSON,
	}), nil
}

// ExecuteApprovedPlan executes the processing steps for an approved execution plan.
func (h *ConnectHandler) ExecuteApprovedPlan(ctx context.Context, req *connect.Request[workerv1.ExecuteApprovedPlanRequest]) (*connect.Response[workerv1.ExecuteApprovedPlanResponse], error) {
	dispatchReq := ExecutePlanRequest{
		JobID:       req.Msg.GetJobId(),
		JobType:     req.Msg.GetJobType(),
		DocumentID:  req.Msg.GetDocumentId(),
		WorkspaceID: req.Msg.GetWorkspaceId(),
		TreeID:      req.Msg.GetTreeId(),
		FileURI:     req.Msg.GetFileUri(),
		Filename:    req.Msg.GetFilename(),
		MimeType:    req.Msg.GetMimeType(),
	}
	h.logger.Info("worker.execute_plan_received", "job_id", req.Msg.GetJobId(), "doc_id", req.Msg.GetDocumentId(), "workspace_id", req.Msg.GetWorkspaceId())
	if err := dispatchReq.Validate(); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := h.processor.Process(ctx, dispatchReq); err != nil {
		if errors.Is(err, ErrApprovalRequired) || errors.Is(err, ErrPlanRejected) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
		return nil, workerConnectError(err)
	}
	return connect.NewResponse(&workerv1.ExecuteApprovedPlanResponse{Status: "ok"}), nil
}

// EvaluateJobArtifact performs semantic quality evaluation on artifacts produced by a completed job.
func (h *ConnectHandler) EvaluateJobArtifact(ctx context.Context, req *connect.Request[workerv1.EvaluateJobArtifactRequest]) (*connect.Response[workerv1.EvaluateJobArtifactResponse], error) {
	if req.Msg.GetJobId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("job_id is required"))
	}

	var result *domain.JobEvaluationResult

	if h.evaluator != nil {
		var err error
		result, err = h.evaluator.Evaluate(ctx, req.Msg.GetJobId())
		if err != nil {
			return nil, workerConnectError(err)
		}
	} else {
		var err error
		result, err = h.jobRepo.EvaluateJob(ctx, req.Msg.GetJobId())
		if err != nil {
			return nil, workerConnectError(err)
		}
	}

	return connect.NewResponse(&workerv1.EvaluateJobArtifactResponse{
		Passed:        result.Passed,
		Status:        result.Status,
		Summary:       result.Summary,
		Score:         result.Score,
		Findings:      result.Findings,
		MutationCount: result.MutationCount,
	}), nil
}

// workerConnectError translates only errors whose meaning is stable at the
// transport boundary. Unknown/provider/database errors remain Internal; the
// observability interceptor masks their details from the caller.
func workerConnectError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, context.Canceled):
		return connect.NewError(connect.CodeCanceled, err)
	case errors.Is(err, context.DeadlineExceeded):
		return connect.NewError(connect.CodeDeadlineExceeded, err)
	case errors.Is(err, domain.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, domain.ErrForbidden):
		return connect.NewError(connect.CodePermissionDenied, err)
	case errors.Is(err, domain.ErrConflict):
		return connect.NewError(connect.CodeAlreadyExists, err)
	case errors.Is(err, domain.ErrNotImplemented):
		return connect.NewError(connect.CodeUnimplemented, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}
