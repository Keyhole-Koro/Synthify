package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
	joblifecycle "github.com/synthify/backend/internal/platform/job/lifecycle"
	jobstatus "github.com/synthify/backend/internal/platform/job/status"
)

// fakeLifecycle captures the ctx and failure handed to TryFailWith so the test
// can assert that failJob detaches from a cancelled request ctx.
type fakeLifecycle struct {
	ctxErrAtCall   error // ctx.Err() captured at the moment TryFailWith runs
	ctxHadDeadline bool
	gotFailure     joblifecycle.Failure
	called         bool
}

func (f *fakeLifecycle) MarkRunning(context.Context, jobstatus.Payload) error { return nil }
func (f *fakeLifecycle) UpdateStage(context.Context, jobstatus.Payload, string) error {
	return nil
}
func (f *fakeLifecycle) Complete(context.Context, jobstatus.Payload, joblifecycle.CompletionOutcome) error {
	return nil
}
func (f *fakeLifecycle) TryFail(context.Context, jobstatus.Payload, error) {}
func (f *fakeLifecycle) TryFailWith(ctx context.Context, _ jobstatus.Payload, failure joblifecycle.Failure) {
	// Capture liveness *during* the call. failJob defers cancel() of the
	// detached ctx, so checking ctx.Err() after failJob returns would always
	// see "cancelled" — the meaningful assertion is that the ctx is alive while
	// the DB/Firestore writes would actually run.
	f.called = true
	f.ctxErrAtCall = ctx.Err()
	_, f.ctxHadDeadline = ctx.Deadline()
	f.gotFailure = failure
}

// failJob must move a job to FAILED even when the request ctx is already
// cancelled (the Cloud Run timeout case). If it forwarded the cancelled ctx, the
// DB + Firestore writes inside TryFailWith would fail and the job would wedge in
// RUNNING forever. failJob therefore detaches via context.WithoutCancel.
func TestFailJob_DetachesFromCancelledContext(t *testing.T) {
	fake := &fakeLifecycle{}
	w := &Worker{lifecycle: fake}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // simulate Cloud Run cancelling the request ctx on timeout

	req := ExecutePlanRequest{JobID: "job1", DocumentID: "doc1", WorkspaceID: "ws1"}
	cause := errors.New("agent run: context canceled")

	w.failJob(ctx, req, jobstatus.Payload{JobID: "job1"}, cause)

	if !fake.called {
		t.Fatal("TryFailWith was not called")
	}
	if fake.ctxErrAtCall != nil {
		t.Fatalf("failJob forwarded a cancelled ctx to TryFailWith: %v", fake.ctxErrAtCall)
	}
	if !fake.ctxHadDeadline {
		t.Error("expected the detached ctx to carry a bounding deadline")
	}
}

// classifyFailure feeds the reason TryFailWith records; a completion-path error
// (the GetDocumentRootItemID "not found" case that wedged jobs in prod) should
// classify as internal, and failJob must still drive the FAILED transition.
func TestFailJob_ForwardsClassifiedReason(t *testing.T) {
	fake := &fakeLifecycle{}
	w := &Worker{lifecycle: fake}

	req := ExecutePlanRequest{JobID: "job1"}
	cause := errors.New("get document root item id for completion: not found")

	w.failJob(context.Background(), req, jobstatus.Payload{JobID: "job1"}, cause)

	if !fake.called {
		t.Fatal("TryFailWith was not called")
	}
	if fake.gotFailure.Reason != jobstatus.FirestoreJobStatusReasonInternal {
		t.Errorf("reason = %q, want internal", fake.gotFailure.Reason)
	}
	if fake.gotFailure.Cause == nil || fake.gotFailure.Cause.Error() != cause.Error() {
		t.Errorf("cause = %v, want %v", fake.gotFailure.Cause, cause)
	}
}

// Guard against the wrap helper drifting: ErrAgentExecution-wrapped causes must
// still classify as agent_error so the frontend can switch on it.
func TestClassifyFailure_AgentExecution(t *testing.T) {
	err := errors.New("boom")
	wrapped := errors.Join(domain.ErrAgentExecution, err)
	if got := classifyFailure(wrapped); got != jobstatus.FirestoreJobStatusReasonAgentError {
		t.Errorf("classifyFailure = %q, want agent_error", got)
	}
}
