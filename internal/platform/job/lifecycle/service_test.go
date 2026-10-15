package joblifecycle

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	jobstatus "github.com/synthify/backend/internal/platform/job/status"
)

type lifecycleRepo struct {
	runningErr    error
	stageErr      error
	completeErr   error
	failErr       error
	runningCalls  int
	stage         string
	failedMessage string
}

func (r *lifecycleRepo) MarkProcessingJobRunning(context.Context, string) error {
	r.runningCalls++
	return r.runningErr
}
func (r *lifecycleRepo) UpdateProcessingJobStage(_ context.Context, _ string, stage string) error {
	r.stage = stage
	return r.stageErr
}
func (r *lifecycleRepo) FailProcessingJob(_ context.Context, _ string, message string) error {
	r.failedMessage = message
	return r.failErr
}
func (r *lifecycleRepo) CompleteProcessingJob(context.Context, string) error { return r.completeErr }

type lifecycleNotifier struct {
	queued      int
	running     int
	stage       string
	failed      *jobstatus.FailureNotification
	completed   *jobstatus.CompletionNotification
	queuedErr   error
	runningErr  error
	stageErr    error
	failedErr   error
	completeErr error
}

func (n *lifecycleNotifier) Queued(context.Context, jobstatus.Payload) error {
	n.queued++
	return n.queuedErr
}
func (n *lifecycleNotifier) Running(context.Context, jobstatus.Payload) error {
	n.running++
	return n.runningErr
}
func (n *lifecycleNotifier) Stage(_ context.Context, _ jobstatus.Payload, stage string) error {
	n.stage = stage
	return n.stageErr
}
func (n *lifecycleNotifier) StageProgress(context.Context, jobstatus.Payload, string, int, string) error {
	return nil
}
func (n *lifecycleNotifier) UpdateFields(context.Context, string, string, jobstatus.UpdateFields) error {
	return nil
}
func (n *lifecycleNotifier) Failed(context.Context, jobstatus.Payload, string) error { return nil }
func (n *lifecycleNotifier) FailedWith(_ context.Context, _ jobstatus.Payload, failure jobstatus.FailureNotification) error {
	n.failed = &failure
	return n.failedErr
}
func (n *lifecycleNotifier) Completed(context.Context, jobstatus.Payload) error { return nil }
func (n *lifecycleNotifier) CompletedWith(_ context.Context, _ jobstatus.Payload, completion jobstatus.CompletionNotification) error {
	n.completed = &completion
	return n.completeErr
}

func lifecycleLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func lifecyclePayload() jobstatus.Payload {
	return jobstatus.Payload{JobID: "job_1", WorkspaceID: "ws_1", DocumentID: "doc_1"}
}

func TestServiceTransitionsPersistAndNotify(t *testing.T) {
	repo := &lifecycleRepo{}
	notifier := &lifecycleNotifier{}
	service := New(repo, notifier, lifecycleLogger(), nil)
	payload := lifecyclePayload()

	if err := service.MarkRunning(context.Background(), payload); err != nil {
		t.Fatalf("MarkRunning() error = %v", err)
	}
	if repo.runningCalls != 1 || notifier.running != 1 {
		t.Fatalf("running calls = repo:%d notifier:%d", repo.runningCalls, notifier.running)
	}
	if err := service.UpdateStage(context.Background(), payload, "extract"); err != nil {
		t.Fatalf("UpdateStage() error = %v", err)
	}
	if repo.stage != "extract" || notifier.stage != "extract" {
		t.Fatalf("stage = repo:%q notifier:%q", repo.stage, notifier.stage)
	}
	if err := service.Complete(context.Background(), payload, CompletionOutcome{TreeChanged: true, StageSummary: []string{"extract"}}); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if notifier.completed == nil || !notifier.completed.TreeChanged || len(notifier.completed.StageSummary) != 1 {
		t.Fatalf("completion = %+v", notifier.completed)
	}
}

func TestServiceReturnsRepositoryTransitionErrors(t *testing.T) {
	want := errors.New("database unavailable")
	repo := &lifecycleRepo{runningErr: want}
	notifier := &lifecycleNotifier{}
	service := New(repo, notifier, lifecycleLogger(), nil)
	if err := service.MarkRunning(context.Background(), lifecyclePayload()); !errors.Is(err, want) {
		t.Fatalf("MarkRunning() error = %v, want %v", err, want)
	}
	if notifier.running != 0 {
		t.Fatalf("notifier running = %d, want 0", notifier.running)
	}
}

func TestTryFailWithPersistsAndNotifiesClassifiedFailure(t *testing.T) {
	repo := &lifecycleRepo{}
	notifier := &lifecycleNotifier{}
	service := New(repo, notifier, lifecycleLogger(), nil)
	cause := errors.New("invalid document")
	service.TryFailWith(context.Background(), lifecyclePayload(), Failure{
		Cause:  cause,
		Reason: jobstatus.FirestoreJobStatusReasonAgentError,
	})
	if repo.failedMessage != cause.Error() {
		t.Fatalf("failed message = %q", repo.failedMessage)
	}
	if notifier.failed == nil || notifier.failed.ErrorMessage != cause.Error() || notifier.failed.Reason != jobstatus.FirestoreJobStatusReasonAgentError {
		t.Fatalf("failure notification = %+v", notifier.failed)
	}
}

func TestNotifyQueuedAndNilNotifierAreSafe(t *testing.T) {
	notifier := &lifecycleNotifier{}
	service := New(&lifecycleRepo{}, notifier, lifecycleLogger(), nil)
	service.NotifyQueued(context.Background(), lifecyclePayload())
	if notifier.queued != 1 {
		t.Fatalf("queued calls = %d, want 1", notifier.queued)
	}
	service = New(&lifecycleRepo{}, nil, lifecycleLogger(), nil)
	service.NotifyQueued(context.Background(), lifecyclePayload())
}
