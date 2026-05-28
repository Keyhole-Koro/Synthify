package joblifecycle

import (
	"context"
	"errors"
	"log/slog"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/synthify/backend/apps/api/internal/domain"
	jobstatus "github.com/synthify/backend/internal/platform/job/status"
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
	logger   *slog.Logger
	nrApp    *newrelic.Application
}

func New(repo Repository, notifier jobstatus.Notifier, logger *slog.Logger, nrApp ...*newrelic.Application) *Service {
	var app *newrelic.Application
	if len(nrApp) > 0 {
		app = nrApp[0]
	}
	return &Service{repo: repo, notifier: notifier, logger: logger, nrApp: app}
}

func (s *Service) NotifyQueued(ctx context.Context, payload jobstatus.Payload) {
	if s.notifier == nil {
		return
	}
	if err := s.notifier.Queued(ctx, payload); err != nil {
		s.logger.Error("jobstatus.notify_queued_failed", "error", err.Error(), "job_id", payload.JobID)
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
		s.logger.Error("jobstatus.notify_running_failed", "error", err.Error(), "job_id", payload.JobID)
	}
	return nil
}

func (s *Service) TryFail(ctx context.Context, payload jobstatus.Payload, cause error) {
	errorMessage := ""
	if cause != nil {
		errorMessage = cause.Error()
	}
	if err := s.repo.FailProcessingJob(ctx, payload.JobID, errorMessage); err != nil {
		s.logger.Error("repository.mark_job_failed_failed", "error", err.Error(), "job_id", payload.JobID)
	}
	s.reportFailure(payload, cause)
	if s.notifier == nil {
		return
	}
	if err := s.notifier.Failed(ctx, payload, errorMessage); err != nil {
		s.logger.Error("jobstatus.notify_failure_failed", "error", err.Error(), "job_id", payload.JobID)
	}
}

func (s *Service) reportFailure(payload jobstatus.Payload, cause error) {
	if s.nrApp == nil {
		return
	}
	errorMessage := ""
	if cause != nil {
		errorMessage = cause.Error()
	}
	s.nrApp.RecordCustomEvent("JobFailed", map[string]any{
		"job_id":       payload.JobID,
		"job_type":     payload.JobType,
		"workspace_id": payload.WorkspaceID,
		"document_id":  payload.DocumentID,
		"tree_id":      payload.TreeID,
		"error":        errorMessage,
	})
	tx := s.nrApp.StartTransaction("job.failed")
	tx.AddAttribute("job_id", payload.JobID)
	tx.AddAttribute("job_type", payload.JobType)
	tx.AddAttribute("workspace_id", payload.WorkspaceID)
	tx.AddAttribute("document_id", payload.DocumentID)
	tx.AddAttribute("tree_id", payload.TreeID)
	if cause == nil {
		cause = errors.New(errorMessage)
	}
	tx.NoticeError(cause)
	tx.End()
}

func (s *Service) Complete(ctx context.Context, payload jobstatus.Payload) error {
	if err := s.repo.CompleteProcessingJob(ctx, payload.JobID); err != nil {
		return err
	}
	if s.notifier == nil {
		return nil
	}
	if err := s.notifier.Completed(ctx, payload); err != nil {
		s.logger.Error("jobstatus.notify_completion_failed", "error", err.Error(), "job_id", payload.JobID)
	}
	return nil
}
