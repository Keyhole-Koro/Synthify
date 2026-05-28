package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	connect "connectrpc.com/connect"
	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/job/lifecycle"
	"github.com/synthify/backend/apps/api/internal/repository"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
	joblog "github.com/synthify/backend/internal/platform/job/log"
)

type JobHandler struct {
	jobs       repository.JobRepository
	approvals  repository.JobApprovalRepository
	jobLogs    repository.JobLogRepository
	workspaces repository.WorkspaceRepository
	documents  repository.DocumentRepository
	lifecycle  *joblifecycle.Service
}

func NewJobHandler(
	jobRepo repository.JobRepository,
	approvalRepo repository.JobApprovalRepository,
	jobLogRepo repository.JobLogRepository,
	lifecycleRepo joblifecycle.Repository,
	workspaceRepo repository.WorkspaceRepository,
	documentRepo repository.DocumentRepository,
	logger *slog.Logger,
) *JobHandler {
	return &JobHandler{
		jobs:       jobRepo,
		approvals:  approvalRepo,
		jobLogs:    jobLogRepo,
		workspaces: workspaceRepo,
		documents:  documentRepo,
		lifecycle:  joblifecycle.New(lifecycleRepo, nil, logger, nil),
	}
}

func (h *JobHandler) GetJobStatus(ctx context.Context, req *connect.Request[appv1.GetJobStatusRequest]) (*connect.Response[appv1.GetJobStatusResponse], error) {
	job, err := h.authorizeAndLoadJob(ctx, req.Msg.GetJobId())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&appv1.GetJobStatusResponse{Job: toProtoJob(job)}), nil
}

func (h *JobHandler) GetJobExecutionPlan(ctx context.Context, req *connect.Request[appv1.GetJobExecutionPlanRequest]) (*connect.Response[appv1.GetJobExecutionPlanResponse], error) {
	if _, err := h.authorizeAndLoadJob(ctx, req.Msg.GetJobId()); err != nil {
		return nil, err
	}
	plan, err := h.jobs.GetJobExecutionPlan(ctx, req.Msg.GetJobId())
	if err != nil {
		return nil, toError(err)
	}
	return connect.NewResponse(&appv1.GetJobExecutionPlanResponse{
		Plan: toProtoExecutionPlan(plan),
	}), nil
}

func (h *JobHandler) ListJobApprovalRequests(ctx context.Context, req *connect.Request[appv1.ListJobApprovalRequestsRequest]) (*connect.Response[appv1.ListJobApprovalRequestsResponse], error) {
	if _, err := h.authorizeAndLoadJob(ctx, req.Msg.GetJobId()); err != nil {
		return nil, err
	}
	requests, err := h.approvals.ListJobApprovalRequests(ctx, req.Msg.GetJobId())
	if err != nil {
		return nil, toError(err)
	}
	res := connect.NewResponse(&appv1.ListJobApprovalRequestsResponse{})
	for _, request := range requests {
		res.Msg.Requests = append(res.Msg.Requests, toProtoApprovalRequest(request))
	}
	return res, nil
}

func (h *JobHandler) RequestJobApproval(ctx context.Context, req *connect.Request[appv1.RequestJobApprovalRequest]) (*connect.Response[appv1.RequestJobApprovalResponse], error) {
	job, err := h.authorizeAndLoadJob(ctx, req.Msg.GetJobId())
	if err != nil {
		return nil, err
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	approval, err := h.lifecycle.RequestApproval(ctx, req.Msg.GetJobId(), userID, req.Msg.GetReason())
	if err != nil {
		return nil, toError(err)
	}
	joblog.FromContext(ctx).Log(ctx, joblog.Event{
		JobID:       job.JobID,
		WorkspaceID: job.WorkspaceID,
		DocumentID:  job.DocumentID,
		Level:       joblog.INFO,
		Event:       "approval.requested",
		Message:     fmt.Sprintf("approval requested by=%s reason=%q", userID, req.Msg.GetReason()),
		Detail:      map[string]any{"by": userID, "reason": req.Msg.GetReason()},
	})
	return connect.NewResponse(&appv1.RequestJobApprovalResponse{Request: toProtoApprovalRequest(approval)}), nil
}

func (h *JobHandler) ApproveJobApproval(ctx context.Context, req *connect.Request[appv1.ApproveJobApprovalRequest]) (*connect.Response[appv1.ApproveJobApprovalResponse], error) {
	job, err := h.authorizeAndLoadJob(ctx, req.Msg.GetJobId())
	if err != nil {
		return nil, err
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	if req.Msg.GetApprovalId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("approval_id is required"))
	}
	if err := h.lifecycle.ApproveApproval(ctx, req.Msg.GetJobId(), req.Msg.GetApprovalId(), userID); err != nil {
		return nil, toError(err)
	}
	joblog.FromContext(ctx).Log(ctx, joblog.Event{
		JobID:       job.JobID,
		WorkspaceID: job.WorkspaceID,
		DocumentID:  job.DocumentID,
		Level:       joblog.INFO,
		Event:       "approval.approved",
		Message:     fmt.Sprintf("approval approved by=%s approval_id=%s", userID, req.Msg.GetApprovalId()),
		Detail:      map[string]any{"by": userID, "approval_id": req.Msg.GetApprovalId()},
	})
	return connect.NewResponse(&appv1.ApproveJobApprovalResponse{Status: "approved"}), nil
}

func (h *JobHandler) RejectJobApproval(ctx context.Context, req *connect.Request[appv1.RejectJobApprovalRequest]) (*connect.Response[appv1.RejectJobApprovalResponse], error) {
	job, err := h.authorizeAndLoadJob(ctx, req.Msg.GetJobId())
	if err != nil {
		return nil, err
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	if req.Msg.GetApprovalId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("approval_id is required"))
	}
	if err := h.lifecycle.RejectApproval(ctx, req.Msg.GetJobId(), req.Msg.GetApprovalId(), userID, req.Msg.GetReason()); err != nil {
		return nil, toError(err)
	}
	joblog.FromContext(ctx).Log(ctx, joblog.Event{
		JobID:       job.JobID,
		WorkspaceID: job.WorkspaceID,
		DocumentID:  job.DocumentID,
		Level:       joblog.WARN,
		Event:       "approval.rejected",
		Message:     fmt.Sprintf("approval rejected by=%s approval_id=%s reason=%q", userID, req.Msg.GetApprovalId(), req.Msg.GetReason()),
		Detail:      map[string]any{"by": userID, "approval_id": req.Msg.GetApprovalId(), "reason": req.Msg.GetReason()},
	})
	return connect.NewResponse(&appv1.RejectJobApprovalResponse{Status: "rejected"}), nil
}

func (h *JobHandler) ListJobMutationLogs(ctx context.Context, req *connect.Request[appv1.ListJobMutationLogsRequest]) (*connect.Response[appv1.ListJobMutationLogsResponse], error) {
	if _, err := h.authorizeAndLoadJob(ctx, req.Msg.GetJobId()); err != nil {
		return nil, err
	}
	logs, err := h.jobLogs.ListJobMutationLogs(ctx, req.Msg.GetJobId())
	if err != nil {
		return nil, toError(err)
	}
	res := connect.NewResponse(&appv1.ListJobMutationLogsResponse{})
	for _, log := range logs {
		res.Msg.Logs = append(res.Msg.Logs, toProtoMutationLog(log))
	}
	return res, nil
}

func (h *JobHandler) ListJobLogs(ctx context.Context, req *connect.Request[appv1.ListJobLogsRequest]) (*connect.Response[appv1.ListJobLogsResponse], error) {
	if _, err := h.authorizeAndLoadJob(ctx, req.Msg.GetJobId()); err != nil {
		return nil, err
	}
	limit := int(req.Msg.GetPageSize())
	if limit <= 0 {
		limit = 500
	}
	logs, nextToken, err := h.jobLogs.ListJobLogs(ctx, req.Msg.GetJobId(), req.Msg.GetPageToken(), limit)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list job logs: %w", err))
	}
	res := connect.NewResponse(&appv1.ListJobLogsResponse{
		NextPageToken: nextToken,
	})
	for _, l := range logs {
		res.Msg.Logs = append(res.Msg.Logs, toProtoJobLog(l))
	}
	return res, nil
}

func (h *JobHandler) SearchJobLogs(ctx context.Context, req *connect.Request[appv1.SearchJobLogsRequest]) (*connect.Response[appv1.SearchJobLogsResponse], error) {
	filter := domain.JobLogSearchFilter{
		Query:         req.Msg.GetQuery(),
		WorkspaceID:   req.Msg.GetWorkspaceId(),
		DocumentID:    req.Msg.GetDocumentId(),
		JobID:         req.Msg.GetJobId(),
		Levels:        req.Msg.GetLevels(),
		Events:        req.Msg.GetEvents(),
		FromTimestamp: req.Msg.GetFromTimestamp(),
		ToTimestamp:   req.Msg.GetToTimestamp(),
		PageToken:     req.Msg.GetPageToken(),
		Limit:         int(req.Msg.GetPageSize()),
	}
	if filter.Limit <= 0 {
		filter.Limit = 200
	}
	if err := h.authorizeLogSearch(ctx, filter); err != nil {
		return nil, err
	}
	logs, nextToken, err := h.jobLogs.SearchJobLogs(ctx, filter)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to search job logs"))
	}
	res := connect.NewResponse(&appv1.SearchJobLogsResponse{
		NextPageToken: nextToken,
	})
	for _, l := range logs {
		res.Msg.Logs = append(res.Msg.Logs, toProtoJobLog(l))
	}
	return res, nil
}

func (h *JobHandler) ListRelatedJobLogs(ctx context.Context, req *connect.Request[appv1.ListRelatedJobLogsRequest]) (*connect.Response[appv1.ListRelatedJobLogsResponse], error) {
	scope := domain.RelatedLogScope(strings.ToLower(req.Msg.GetScope().String()))
	// Handle enum string mapping if needed, but RelatedLogScopeJob etc match the lowercase enum names in domain
	switch req.Msg.GetScope() {
	case appv1.RelatedLogScope_RELATED_LOG_SCOPE_JOB:
		scope = domain.RelatedLogScopeJob
	case appv1.RelatedLogScope_RELATED_LOG_SCOPE_DOCUMENT:
		scope = domain.RelatedLogScopeDocument
	case appv1.RelatedLogScope_RELATED_LOG_SCOPE_WORKSPACE:
		scope = domain.RelatedLogScopeWorkspace
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("unsupported scope"))
	}

	if err := h.authorizeRelatedLogSearch(ctx, scope, req.Msg.GetWorkspaceId(), req.Msg.GetDocumentId(), req.Msg.GetJobId()); err != nil {
		return nil, err
	}
	limit := int(req.Msg.GetPageSize())
	if limit <= 0 {
		limit = 100
	}
	groups, nextToken, err := h.jobLogs.ListRelatedJobLogs(ctx, scope, req.Msg.GetWorkspaceId(), req.Msg.GetDocumentId(), req.Msg.GetJobId(), req.Msg.GetPageToken(), limit)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to list related job logs"))
	}
	res := connect.NewResponse(&appv1.ListRelatedJobLogsResponse{
		NextPageToken: nextToken,
	})
	for _, g := range groups {
		res.Msg.Groups = append(res.Msg.Groups, toProtoJobLogGroup(g))
	}
	return res, nil
}

func (h *JobHandler) ListAllJobs(ctx context.Context, _ *connect.Request[appv1.ListAllJobsRequest]) (*connect.Response[appv1.ListAllJobsResponse], error) {
	// 全 workspace 横断のため admin 権限を要求。
	// monitor など読み取り専用ツールは API を経由せず Postgres を直接参照する設計に
	// 切り替えたため、anonymous バイパスは存在しない。
	if _, err := requireAdminPrincipal(ctx); err != nil {
		return nil, err
	}
	jobs, err := h.jobs.ListAllJobs(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	res := connect.NewResponse(&appv1.ListAllJobsResponse{})
	for _, job := range jobs {
		res.Msg.Jobs = append(res.Msg.Jobs, toProtoJob(job))
	}
	return res, nil
}

func (h *JobHandler) authorizeAndLoadJob(ctx context.Context, jobID string) (*domain.DocumentProcessingJob, error) {
	// 認証を先にチェック。job_id 空 / job not found を未認証ユーザーに返さないため。
	if _, err := requireUserID(ctx); err != nil {
		return nil, err
	}
	if jobID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("job_id is required"))
	}
	job, err := h.jobs.GetProcessingJob(ctx, jobID)
	if err != nil {
		return nil, toError(err)
	}
	if err := authorizeDocument(ctx, h.workspaces, h.documents, job.DocumentID, ""); err != nil {
		return nil, err
	}
	return job, nil
}

func (h *JobHandler) authorizeLogSearch(ctx context.Context, filter domain.JobLogSearchFilter) error {
	switch {
	case filter.JobID != "":
		_, err := h.authorizeAndLoadJob(ctx, filter.JobID)
		return err
	case filter.DocumentID != "":
		return authorizeDocument(ctx, h.workspaces, h.documents, filter.DocumentID, filter.WorkspaceID)
	case filter.WorkspaceID != "":
		return authorizeWorkspace(ctx, h.workspaces, filter.WorkspaceID)
	default:
		return connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id, document_id, or job_id is required"))
	}
}

func (h *JobHandler) authorizeRelatedLogSearch(ctx context.Context, scope domain.RelatedLogScope, workspaceID, documentID, jobID string) error {
	switch scope {
	case domain.RelatedLogScopeJob:
		if jobID == "" {
			return connect.NewError(connect.CodeInvalidArgument, errors.New("job_id is required"))
		}
		_, err := h.authorizeAndLoadJob(ctx, jobID)
		return err
	case domain.RelatedLogScopeDocument:
		if documentID == "" {
			return connect.NewError(connect.CodeInvalidArgument, errors.New("document_id is required"))
		}
		return authorizeDocument(ctx, h.workspaces, h.documents, documentID, workspaceID)
	case domain.RelatedLogScopeWorkspace:
		if workspaceID == "" {
			return connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id is required"))
		}
		return authorizeWorkspace(ctx, h.workspaces, workspaceID)
	default:
		return connect.NewError(connect.CodeInvalidArgument, errors.New("scope must be job, document, or workspace"))
	}
}

func toProtoJob(job *domain.DocumentProcessingJob) *appv1.Job {
	if job == nil {
		return nil
	}
	return &appv1.Job{
		JobId:        job.JobID,
		DocumentId:   job.DocumentID,
		WorkspaceId:  job.WorkspaceID,
		Type:         job.JobType,
		Status:       job.Status,
		CreatedAt:    job.CreatedAt,
		CompletedAt:  job.UpdatedAt,
		ErrorMessage: job.ErrorMessage,
	}
}

func toProtoApprovalRequest(req *domain.JobApprovalRequest) *appv1.JobApprovalRequest {
	if req == nil {
		return nil
	}
	return &appv1.JobApprovalRequest{
		ApprovalId:          req.ApprovalID,
		JobId:               req.JobID,
		PlanId:              req.PlanID,
		Status:              req.Status,
		RequestedOperations: req.RequestedOperations,
		Reason:              req.Reason,
		RiskTier:            req.RiskTier,
		RequestedBy:         req.RequestedBy,
		ReviewedBy:          req.ReviewedBy,
		RequestedAt:         req.RequestedAt,
		ReviewedAt:          req.ReviewedAt,
	}
}

func toProtoMutationLog(log *domain.JobMutationLog) *appv1.JobMutationLog {
	if log == nil {
		return nil
	}
	return &appv1.JobMutationLog{
		MutationId:     log.MutationID,
		JobId:          log.JobID,
		TargetType:     log.TargetType,
		TargetId:       log.TargetID,
		MutationType:   log.MutationType,
		RiskTier:       log.RiskTier,
		BeforeJson:     log.BeforeJSON,
		AfterJson:      log.AfterJSON,
		ProvenanceJson: log.ProvenanceJSON,
		CreatedAt:      log.CreatedAt,
	}
}

func toProtoExecutionPlan(plan *domain.JobExecutionPlan) *appv1.JobExecutionPlan {
	if plan == nil {
		return nil
	}
	return &appv1.JobExecutionPlan{
		PlanId:    plan.PlanID,
		JobId:     plan.JobID,
		Status:    plan.Status,
		Summary:   plan.Summary,
		PlanJson:  plan.PlanJSON,
		CreatedBy: plan.CreatedBy,
		CreatedAt: plan.CreatedAt,
		UpdatedAt: plan.UpdatedAt,
	}
}

func toProtoJobLog(log *domain.JobLog) *appv1.JobLog {
	if log == nil {
		return nil
	}
	return &appv1.JobLog{
		Timestamp:   log.Timestamp,
		Level:       log.Level,
		Event:       log.Event,
		Message:     log.Message,
		DetailJson:  log.DetailJSON,
		Source:      log.Source,
		SourceId:    log.SourceID,
		JobId:       log.JobID,
		DocumentId:  log.DocumentID,
		WorkspaceId: log.WorkspaceID,
	}
}

func toProtoJobLogJob(job *domain.JobLogJob) *appv1.JobLogJob {
	if job == nil {
		return nil
	}
	logs := make([]*appv1.JobLog, 0, len(job.Logs))
	for _, l := range job.Logs {
		logs = append(logs, toProtoJobLog(l))
	}
	return &appv1.JobLogJob{
		JobId:     job.JobID,
		Status:    job.Status,
		CreatedAt: job.CreatedAt,
		Logs:      logs,
	}
}

func toProtoJobLogGroup(group *domain.JobLogGroup) *appv1.JobLogGroup {
	if group == nil {
		return nil
	}
	jobs := make([]*appv1.JobLogJob, 0, len(group.Jobs))
	for _, j := range group.Jobs {
		jobs = append(jobs, toProtoJobLogJob(j))
	}
	return &appv1.JobLogGroup{
		WorkspaceId: group.WorkspaceID,
		DocumentId:  group.DocumentID,
		Jobs:        jobs,
	}
}
