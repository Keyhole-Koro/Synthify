package worker

import (
	"context"
	"errors"
	"log/slog"

	connect "connectrpc.com/connect"
	"github.com/synthify/backend/apps/worker/pkg/worker/chat"
	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
	workerv1 "github.com/synthify/backend/internal/gen/synthify/worker/v1"
)

// ChatTurnRunner generates one dialogue turn and publishes it to Firestore.
type ChatTurnRunner interface {
	Run(ctx context.Context, req chat.Request) error
}

type ConnectHandler struct {
	processor interface {
		Process(ctx context.Context, req ExecutePlanRequest) error
	}
	jobRepo   Repository
	planner   *Planner
	evaluator *JobEvaluator
	chat      ChatTurnRunner
	logger    *slog.Logger
}

func NewConnectHandler(processor interface {
	Process(ctx context.Context, req ExecutePlanRequest) error
}, jobRepo Repository, planner *Planner, evaluator *JobEvaluator, chatRunner ChatTurnRunner, logger *slog.Logger) *ConnectHandler {
	return &ConnectHandler{
		processor: processor,
		jobRepo:   jobRepo,
		planner:   planner,
		evaluator: evaluator,
		chat:      chatRunner,
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

// RunChatTurn generates one dialogue turn.
//
// The RPC returns once generation finishes, but the answer does not travel in
// the response: it is written to the Firestore turn document the browser is
// already subscribed to. That is what lets a section appear as soon as it is
// produced instead of after the whole turn.
func (h *ConnectHandler) RunChatTurn(ctx context.Context, req *connect.Request[workerv1.RunChatTurnRequest]) (*connect.Response[workerv1.RunChatTurnResponse], error) {
	if h.chat == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("chat is not configured on this worker"))
	}
	msg := req.Msg
	if msg.GetUserId() == "" || msg.GetChatId() == "" || msg.GetTurnId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id, chat_id and turn_id are required"))
	}

	h.logger.Info("worker.run_chat_turn_received",
		"chat_id", msg.GetChatId(),
		"turn_id", msg.GetTurnId(),
		"workspace_id", msg.GetWorkspaceId(),
		"candidates", len(msg.GetCandidates()),
	)

	history := make([]chat.Message, 0, len(msg.GetHistory()))
	for _, m := range msg.GetHistory() {
		history = append(history, chat.Message{Role: chatRoleName(m.GetRole()), Text: m.GetText()})
	}
	candidates := make([]chat.ContextPaper, 0, len(msg.GetCandidates()))
	for _, c := range msg.GetCandidates() {
		candidates = append(candidates, chat.ContextPaper{
			PaperID:     c.GetPaperId(),
			Title:       c.GetTitle(),
			Description: c.GetDescription(),
			Content:     c.GetContent(),
		})
	}

	if err := h.chat.Run(ctx, chat.Request{
		UserID:            msg.GetUserId(),
		WorkspaceID:       msg.GetWorkspaceId(),
		ChatID:            msg.GetChatId(),
		TurnID:            msg.GetTurnId(),
		PaperID:           msg.GetPaperId(),
		History:           history,
		Candidates:        candidates,
		PinnedContextIDs:  msg.GetPinnedContextIds(),
		AutoSelectContext: msg.GetAutoSelectContext(),
	}); err != nil {
		// The failure is already recorded on the turn document, so the browser
		// sees it regardless of what this response says.
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&workerv1.RunChatTurnResponse{Status: "ok"}), nil
}

func chatRoleName(r workerv1.ChatRole) string {
	if r == workerv1.ChatRole_CHAT_ROLE_ASSISTANT {
		return "assistant"
	}
	return "user"
}
