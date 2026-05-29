package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
)

type fakeProcessor struct {
	gotReq ExecutePlanRequest
	err    error
	called bool
}

func (f *fakeProcessor) Process(ctx context.Context, req ExecutePlanRequest) error {
	f.gotReq = req
	f.called = true
	return f.err
}

type fakePlanner struct {
	gotReq ExecutePlanRequest
	plan   *domain.JobExecutionPlan
	err    error
	called bool
}

func (f *fakePlanner) GenerateExecutionPlan(ctx context.Context, req ExecutePlanRequest) (*domain.JobExecutionPlan, error) {
	f.gotReq = req
	f.called = true
	return f.plan, f.err
}

func newDiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func validPayload() domain.ExecutePlanRequest {
	return domain.ExecutePlanRequest{
		JobID:       "job_1",
		DocumentID:  "doc_1",
		WorkspaceID: "ws_1",
		TreeID:      "ws_1",
		JobType:     "PROCESS_DOCUMENT",
	}
}

func postJSON(t *testing.T, handler http.Handler, body any) *httptest.ResponseRecorder {
	t.Helper()
	buf := new(bytes.Buffer)
	if body != nil {
		if err := json.NewEncoder(buf).Encode(body); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/internal/dispatch-job", buf)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestInternalDispatch_RejectsWrongMethod(t *testing.T) {
	h := NewInternalDispatchHandler(&fakeProcessor{}, &fakePlanner{}, newDiscardLogger())
	req := httptest.NewRequest(http.MethodGet, "/internal/dispatch-job", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestInternalDispatch_DecodesAndExecutesApprovedPlan(t *testing.T) {
	proc := &fakeProcessor{}
	pl := &fakePlanner{}
	h := NewInternalDispatchHandler(proc, pl, newDiscardLogger())

	rec := postJSON(t, h, internalDispatchRequest{
		Procedure: "ExecuteApprovedPlan",
		Payload:   validPayload(),
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !proc.called {
		t.Fatalf("processor not invoked")
	}
	if pl.called {
		t.Fatalf("planner unexpectedly invoked for ExecuteApprovedPlan")
	}
	if proc.gotReq.JobID != "job_1" {
		t.Fatalf("payload not threaded through: %+v", proc.gotReq)
	}
}

func TestInternalDispatch_DecodesAndGeneratesPlan(t *testing.T) {
	proc := &fakeProcessor{}
	pl := &fakePlanner{plan: &domain.JobExecutionPlan{JobID: "job_1", PlanID: "plan_job_1"}}
	h := NewInternalDispatchHandler(proc, pl, newDiscardLogger())

	rec := postJSON(t, h, internalDispatchRequest{
		Procedure: "GenerateExecutionPlan",
		Payload:   validPayload(),
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !pl.called {
		t.Fatalf("planner not invoked")
	}
	if proc.called {
		t.Fatalf("processor unexpectedly invoked for GenerateExecutionPlan")
	}
}

func TestInternalDispatch_4xxOnInvalidJSON(t *testing.T) {
	h := NewInternalDispatchHandler(&fakeProcessor{}, &fakePlanner{}, newDiscardLogger())
	req := httptest.NewRequest(http.MethodPost, "/internal/dispatch-job", bytes.NewBufferString("{not json"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad JSON, got %d", rec.Code)
	}
}

func TestInternalDispatch_4xxOnUnknownProcedure(t *testing.T) {
	proc := &fakeProcessor{}
	pl := &fakePlanner{}
	h := NewInternalDispatchHandler(proc, pl, newDiscardLogger())

	rec := postJSON(t, h, internalDispatchRequest{
		Procedure: "DoSomethingElse",
		Payload:   validPayload(),
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown procedure, got %d", rec.Code)
	}
	if proc.called || pl.called {
		t.Fatalf("handlers unexpectedly invoked for unknown procedure")
	}
}

func TestInternalDispatch_4xxOnInvalidPayload(t *testing.T) {
	h := NewInternalDispatchHandler(&fakeProcessor{}, &fakePlanner{}, newDiscardLogger())
	bad := validPayload()
	bad.JobID = ""
	rec := postJSON(t, h, internalDispatchRequest{
		Procedure: "ExecuteApprovedPlan",
		Payload:   bad,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing job_id, got %d", rec.Code)
	}
}

func TestInternalDispatch_PlanRejectionIs4xx(t *testing.T) {
	// A rejected plan is a terminal decision, not a transient error. Cloud
	// Tasks should NOT retry it, so the handler must return a 4xx.
	proc := &fakeProcessor{err: ErrPlanRejected}
	h := NewInternalDispatchHandler(proc, &fakePlanner{}, newDiscardLogger())

	rec := postJSON(t, h, internalDispatchRequest{
		Procedure: "ExecuteApprovedPlan",
		Payload:   validPayload(),
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for plan rejected, got %d", rec.Code)
	}
}

func TestInternalDispatch_ApprovalRequiredIs4xx(t *testing.T) {
	proc := &fakeProcessor{err: ErrApprovalRequired}
	h := NewInternalDispatchHandler(proc, &fakePlanner{}, newDiscardLogger())

	rec := postJSON(t, h, internalDispatchRequest{
		Procedure: "ExecuteApprovedPlan",
		Payload:   validPayload(),
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for approval required, got %d", rec.Code)
	}
}

func TestInternalDispatch_TransientErrorIs5xx(t *testing.T) {
	// Anything that is not a domain-classified terminal error must be 5xx
	// so Cloud Tasks backs off and retries.
	proc := &fakeProcessor{err: errors.New("postgres connection refused")}
	h := NewInternalDispatchHandler(proc, &fakePlanner{}, newDiscardLogger())

	rec := postJSON(t, h, internalDispatchRequest{
		Procedure: "ExecuteApprovedPlan",
		Payload:   validPayload(),
	})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for transient error, got %d", rec.Code)
	}
}

func TestInternalDispatch_PlannerErrorIs5xx(t *testing.T) {
	pl := &fakePlanner{err: errors.New("llm timeout")}
	h := NewInternalDispatchHandler(&fakeProcessor{}, pl, newDiscardLogger())

	rec := postJSON(t, h, internalDispatchRequest{
		Procedure: "GenerateExecutionPlan",
		Payload:   validPayload(),
	})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for planner failure, got %d", rec.Code)
	}
}
