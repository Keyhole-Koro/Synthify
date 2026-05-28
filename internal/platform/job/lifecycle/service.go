// Package joblifecycle owns the document-processing-job state machine
// transitions (queued / running / succeeded / failed) that both api and worker
// participate in. It used to live as a duplicated copy under apps/api and
// apps/worker; hoisting it here keeps the transitions in lockstep across
// services.
//
// The package exposes two interfaces — QueuedNotifier (api side) and
// RuntimeController (worker side) — so callers can only invoke the
// transitions they are responsible for. See docs/architecture/target-design.md
// section 2 for the responsibility split.
//
// Approval-related repository calls (RequestJobApproval and friends) are
// intentionally not part of this package. They live on the api handler and
// repository layer because they are simple pass-throughs that the worker
// never invokes; routing them through this package would create dead code on
// the worker side and inflate the cross-service surface area for no benefit.
package joblifecycle

import (
	"context"
	"errors"
	"log/slog"

	"github.com/newrelic/go-agent/v3/newrelic"
	jobstatus "github.com/synthify/backend/internal/platform/job/status"
)

// Repository is the minimal set of state-machine writes that joblifecycle
// needs. Both api and worker Postgres stores already implement these methods.
type Repository interface {
	MarkProcessingJobRunning(ctx context.Context, jobID string) error
	UpdateProcessingJobStage(ctx context.Context, jobID, stage string) error
	FailProcessingJob(ctx context.Context, jobID, errorMessage string) error
	CompleteProcessingJob(ctx context.Context, jobID string) error
}

// QueuedNotifier is the slice of Service exposed to the api side. The api
// creates jobs (NotifyQueued) and routes approval flow. It also has to be
// able to drive a job to failed when the dispatch RPC to worker itself fails,
// because at that point the worker was never reached and only the api knows
// the job aborted — hence TryFail is shared with RuntimeController.
type QueuedNotifier interface {
	NotifyQueued(ctx context.Context, payload jobstatus.Payload)
	TryFail(ctx context.Context, payload jobstatus.Payload, cause error)
}

// RuntimeController is the slice of Service exposed to the worker. The worker
// drives a running job to its terminal state. It must not be able to call
// NotifyQueued — only the api creates jobs.
type RuntimeController interface {
	MarkRunning(ctx context.Context, payload jobstatus.Payload) error
	UpdateStage(ctx context.Context, payload jobstatus.Payload, stage string) error
	Complete(ctx context.Context, payload jobstatus.Payload) error
	TryFail(ctx context.Context, payload jobstatus.Payload, cause error)
}

// Service satisfies both QueuedNotifier and RuntimeController. Construct it
// once per process and inject the appropriate narrower interface into each
// caller so the type system enforces who can do what.
type Service struct {
	repo     Repository
	notifier jobstatus.Notifier
	logger   *slog.Logger
	nrApp    *newrelic.Application
}

// New constructs a Service. nrApp may be nil when New Relic reporting is not
// configured; incident reporting becomes a no-op rather than panicking.
func New(repo Repository, notifier jobstatus.Notifier, logger *slog.Logger, nrApp *newrelic.Application) *Service {
	return &Service{repo: repo, notifier: notifier, logger: logger, nrApp: nrApp}
}

// --- QueuedNotifier ----------------------------------------------------------

func (s *Service) NotifyQueued(ctx context.Context, payload jobstatus.Payload) {
	if s.notifier == nil {
		return
	}
	if err := s.notifier.Queued(ctx, payload); err != nil {
		s.logger.Error("jobstatus.notify_queued_failed", "error", err.Error(), "job_id", payload.JobID)
	}
}

// --- RuntimeController -------------------------------------------------------

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

func (s *Service) UpdateStage(ctx context.Context, payload jobstatus.Payload, stage string) error {
	if err := s.repo.UpdateProcessingJobStage(ctx, payload.JobID, stage); err != nil {
		return err
	}
	if s.notifier == nil {
		return nil
	}
	if err := s.notifier.Stage(ctx, payload, stage); err != nil {
		s.logger.Error("jobstatus.notify_stage_failed", "error", err.Error(), "job_id", payload.JobID, "stage", stage)
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

// --- internals ---------------------------------------------------------------

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
