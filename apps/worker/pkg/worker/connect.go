package worker

import (
	"context"
	"errors"
	"log/slog"

	connect "connectrpc.com/connect"
	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
	workerv1 "github.com/synthify/backend/internal/gen/synthify/worker/v1"
)

type ConnectHandler struct {
	processor interface {
		Process(ctx context.Context, req ExecutePlanRequest) error
	}
	jobRepo   Repository
	planner   *Planner
	evaluator *JobEvaluator
	logger    *slog.Logger
}

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

func (h *ConnectHandler) GenerateExecutionPlan(ctx context.Context, req *connect.Request[workerv1.GenerateExecutionPlanRequest]) (*connect.Response[workerv1.GenerateExecutionPlanResponse], error) {
	if req.Msg.GetJobId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("job_id is required"))
	}
	h.logger.Info("worker.generate_plan_received", "job_id", req.Msg.GetJobId(), "doc_id", req.Msg.GetDocumentId())
	plan, err := h.planner.GenerateExecutionPlan(ctx, ExecutePlanRequest{
		JobID:       req.Msg.GetJobId(),
		JobType:     req.Msg.GetJobType(),
		DocumentID:  req.Msg.GetDocumentId(),
		WorkspaceID: req.Msg.GetWorkspaceId(),
		TreeID:      req.Msg.GetTreeId(),
		Filename:    req.Msg.GetFilename(),
		MimeType:    req.Msg.GetMimeType(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewResponse(&workerv1.GenerateExecutionPlanResponse{
		PlanId:   plan.PlanID,
		JobId:    plan.JobID,
		Status:   plan.Status,
		Summary:  plan.Summary,
		PlanJson: plan.PlanJSON,
	}), nil
}

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
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&workerv1.ExecuteApprovedPlanResponse{Status: "ok"}), nil
}

func (h *ConnectHandler) EvaluateJobArtifact(ctx context.Context, req *connect.Request[workerv1.EvaluateJobArtifactRequest]) (*connect.Response[workerv1.EvaluateJobArtifactResponse], error) {
	if req.Msg.GetJobId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("job_id is required"))
	}

	var result *domain.JobEvaluationResult

	if h.evaluator != nil {
		var err error
		result, err = h.evaluator.Evaluate(ctx, req.Msg.GetJobId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	} else {
		var err error
		result, err = h.jobRepo.EvaluateJob(ctx, req.Msg.GetJobId())
		if err != nil {
			return nil, connect.NewError(connect.CodeNotFound, err)
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
