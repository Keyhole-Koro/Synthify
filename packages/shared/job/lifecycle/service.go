package joblifecycle

import (
	"context"

	"github.com/synthify/backend/packages/shared/applog"
	"github.com/synthify/backend/packages/shared/domain"
	"github.com/synthify/backend/packages/shared/job/status"
)

type Repository interface {
	RequestJobApproval(ctx context.Context, jobID, requestedBy, reason string) (*domain.JobApprovalRequest, error)
	ApproveJobApproval(ctx context.Context, jobID, approvalID, reviewedBy string) error
	RejectJobApproval(ctx context.Context, jobID, approvalID, reviewedBy, reason string) error
	MarkProcessingJobRunning(ctx context.Context, jobID string) error
	FailProcessingJob(ctx context.Context, jobID, errorMessage string) error
	CompleteProcessingJob(ctx context.Context, jobID string) error
}

type Service struct {
	repo     Repository
	notifier jobstatus.Notifier
	logger   applog.Logger
}

func New(repo Repository, notifier jobstatus.Notifier, logger applog.Logger) *Service {
	if logger == nil {
		logger = applog.NoopLogger{}
	}
	return &Service{repo: repo, notifier: notifier, logger: logger}
}

func (s *Service) NotifyQueued(ctx context.Context, payload jobstatus.Payload) {
	if s.notifier == nil {
		return
	}
	if err := s.notifier.Queued(ctx, payload); err != nil {
		s.logger.Error(ctx, "jobstatus.notify_queued_failed", err, map[string]any{"job_id": payload.JobID})
	}
}

func (s *Service) RequestApproval(ctx context.Context, jobID, requestedBy, reason string) (*domain.JobApprovalRequest, error) {
	return s.repo.RequestJobApproval(ctx, jobID, requestedBy, reason)
}

func (s *Service) ApproveApproval(ctx context.Context, jobID, approvalID, reviewedBy string) error {
	return s.repo.ApproveJobApproval(ctx, jobID, approvalID, reviewedBy)
}

func (s *Service) RejectApproval(ctx context.Context, jobID, approvalID, reviewedBy, reason string) error {
	return s.repo.RejectJobApproval(ctx, jobID, approvalID, reviewedBy, reason)
}

func (s *Service) MarkRunning(ctx context.Context, payload jobstatus.Payload) error {
	if err := s.repo.MarkProcessingJobRunning(ctx, payload.JobID); err != nil {
		return err
	}
	if s.notifier == nil {
		return nil
	}
	if err := s.notifier.Running(ctx, payload); err != nil {
		s.logger.Error(ctx, "jobstatus.notify_running_failed", err, map[string]any{"job_id": payload.JobID})
	}
	return nil
}

func (s *Service) TryFail(ctx context.Context, payload jobstatus.Payload, errorMessage string) {
	if err := s.repo.FailProcessingJob(ctx, payload.JobID, errorMessage); err != nil {
		s.logger.Error(ctx, "repository.mark_job_failed_failed", err, map[string]any{"job_id": payload.JobID})
	}
	if s.notifier == nil {
		return
	}
	if err := s.notifier.Failed(ctx, payload, errorMessage); err != nil {
		s.logger.Error(ctx, "jobstatus.notify_failure_failed", err, map[string]any{"job_id": payload.JobID})
	}
}

func (s *Service) Complete(ctx context.Context, payload jobstatus.Payload) error {
	if err := s.repo.CompleteProcessingJob(ctx, payload.JobID); err != nil {
		return err
	}
	if s.notifier == nil {
		return nil
	}
	if err := s.notifier.Completed(ctx, payload); err != nil {
		s.logger.Error(ctx, "jobstatus.notify_completion_failed", err, map[string]any{"job_id": payload.JobID})
	}
	return nil
}
