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
	Complete(ctx context.Context, payload jobstatus.Payload, outcome CompletionOutcome) error
	TryFail(ctx context.Context, payload jobstatus.Payload, cause error)
	TryFailWith(ctx context.Context, payload jobstatus.Payload, failure Failure)
}

// CompletionOutcome carries the metadata the worker collects while a job
// runs so it can be reflected onto the Firestore mirror without an extra
// round-trip. TreeChanged tells the frontend to refetch the workspace tree
// and diff-reveal new nodes; StageSummary is optional human-readable progress.
type CompletionOutcome struct {
	TreeChanged  bool
	StageSummary []string
}

// Failure pairs an error with a classified reason so the Firestore mirror
// can hand the frontend a switchable value alongside the human-readable
// message. ReasonInternal is the safe fallback for unknown causes.
type Failure struct {
	Cause  error
	Reason jobstatus.FirestoreJobStatusReason
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

// TryFail is the shorthand for an unclassified failure: it routes through
// TryFailWith with reason=internal so callers that do not (or cannot)
// classify the cause still produce a Firestore-conformant notification.
func (s *Service) TryFail(ctx context.Context, payload jobstatus.Payload, cause error) {
	s.TryFailWith(ctx, payload, Failure{Cause: cause, Reason: jobstatus.FirestoreJobStatusReasonInternal})
}

func (s *Service) TryFailWith(ctx context.Context, payload jobstatus.Payload, failure Failure) {
	errorMessage := ""
	if failure.Cause != nil {
		errorMessage = failure.Cause.Error()
	}
	reason := failure.Reason
	if reason == "" {
		reason = jobstatus.FirestoreJobStatusReasonInternal
	}
	if err := s.repo.FailProcessingJob(ctx, payload.JobID, errorMessage); err != nil {
		s.logger.Error("repository.mark_job_failed_failed", "error", err.Error(), "job_id", payload.JobID)
	}
	s.reportFailure(payload, failure.Cause, reason)
	if s.notifier == nil {
		return
	}
	if err := s.notifier.FailedWith(ctx, payload, jobstatus.FailureNotification{
		ErrorMessage: errorMessage,
		Reason:       reason,
	}); err != nil {
		s.logger.Error("jobstatus.notify_failure_failed", "error", err.Error(), "job_id", payload.JobID)
		s.reportNotificationFailure(payload, "failure", err)
	}
}

func (s *Service) Complete(ctx context.Context, payload jobstatus.Payload, outcome CompletionOutcome) error {
	if err := s.repo.CompleteProcessingJob(ctx, payload.JobID); err != nil {
		return err
	}
	if s.notifier == nil {
		return nil
	}
	if err := s.notifier.CompletedWith(ctx, payload, jobstatus.CompletionNotification{
		TreeChanged:  outcome.TreeChanged,
		StageSummary: outcome.StageSummary,
	}); err != nil {
		s.logger.Error("jobstatus.notify_completion_failed", "error", err.Error(), "job_id", payload.JobID)
		s.reportNotificationFailure(payload, "completion", err)
	}
	return nil
}

// reportNotificationFailure surfaces a failed Firestore status write to New
// Relic. The frontend learns of job completion/failure only via the Firestore
// onSnapshot() subscription (see README), so a dropped notification leaves the
// UI hanging on an otherwise-finished job — a silent, user-visible failure that
// the slog line alone would never alert on. no-op when NR is disabled.
func (s *Service) reportNotificationFailure(payload jobstatus.Payload, kind string, cause error) {
	if s.nrApp == nil || cause == nil {
		return
	}
	s.nrApp.RecordCustomEvent("NotificationFailed", map[string]any{
		"job_id":       payload.JobID,
		"job_type":     payload.JobType,
		"workspace_id": payload.WorkspaceID,
		"document_id":  payload.DocumentID,
		"kind":         kind,
		"error":        cause.Error(),
	})
	tx := s.nrApp.StartTransaction("job.notification_failed")
	tx.AddAttribute("job_id", payload.JobID)
	tx.AddAttribute("kind", kind)
	tx.NoticeError(cause)
	tx.End()
}

// --- internals ---------------------------------------------------------------

func (s *Service) reportFailure(payload jobstatus.Payload, cause error, reason jobstatus.FirestoreJobStatusReason) {
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
		"reason":       string(reason),
		"error":        errorMessage,
	})
	tx := s.nrApp.StartTransaction("job.failed")
	tx.AddAttribute("job_id", payload.JobID)
	tx.AddAttribute("job_type", payload.JobType)
	tx.AddAttribute("workspace_id", payload.WorkspaceID)
	tx.AddAttribute("document_id", payload.DocumentID)
	tx.AddAttribute("tree_id", payload.TreeID)
	tx.AddAttribute("reason", string(reason))
	if cause == nil {
		cause = errors.New(errorMessage)
	}
	tx.NoticeError(cause)
	tx.End()
}
