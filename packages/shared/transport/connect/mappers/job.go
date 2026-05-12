package mappers

import (
	"github.com/synthify/backend/packages/shared/domain"
	treev1 "github.com/synthify/backend/packages/shared/gen/synthify/tree/v1"
)

func ToProtoJob(job *domain.DocumentProcessingJob) *treev1.Job {
	if job == nil {
		return nil
	}
	return &treev1.Job{
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

func ToProtoApprovalRequest(req *domain.JobApprovalRequest) *treev1.JobApprovalRequest {
	if req == nil {
		return nil
	}
	return &treev1.JobApprovalRequest{
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

func ToProtoMutationLog(log *domain.JobMutationLog) *treev1.JobMutationLog {
	if log == nil {
		return nil
	}
	return &treev1.JobMutationLog{
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

func ToProtoExecutionPlan(plan *domain.JobExecutionPlan) *treev1.JobExecutionPlan {
	if plan == nil {
		return nil
	}
	return &treev1.JobExecutionPlan{
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

func ToProtoJobLog(log *domain.JobLog) *treev1.JobLog {
	if log == nil {
		return nil
	}
	return &treev1.JobLog{
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

func ToProtoJobLogJob(job *domain.JobLogJob) *treev1.JobLogJob {
	if job == nil {
		return nil
	}
	logs := make([]*treev1.JobLog, 0, len(job.Logs))
	for _, l := range job.Logs {
		logs = append(logs, ToProtoJobLog(l))
	}
	return &treev1.JobLogJob{
		JobId:     job.JobID,
		Status:    job.Status,
		CreatedAt: job.CreatedAt,
		Logs:      logs,
	}
}

func ToProtoJobLogGroup(group *domain.JobLogGroup) *treev1.JobLogGroup {
	if group == nil {
		return nil
	}
	jobs := make([]*treev1.JobLogJob, 0, len(group.Jobs))
	for _, j := range group.Jobs {
		jobs = append(jobs, ToProtoJobLogJob(j))
	}
	return &treev1.JobLogGroup{
		WorkspaceId: group.WorkspaceID,
		DocumentId:  group.DocumentID,
		Jobs:        jobs,
	}
}
