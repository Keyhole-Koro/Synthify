package worker

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
)

// InternalDispatchHandler is the HTTP entrypoint Cloud Tasks calls into.
//
// Cloud Tasks delivers a plain JSON POST with the procedure name and the
// ExecutePlanRequest payload. Using a small dedicated endpoint instead of the
// existing Connect RPCs avoids the impedance mismatch between Cloud Tasks's
// raw HTTP body and Connect's framing.
//
// Authentication is enforced at the Cloud Run layer: the service is deployed
// with allow_unauthenticated=false, and only the Cloud Tasks service account
// (and the API SA during local override) holds roles/run.invoker. The OIDC
// token Cloud Tasks attaches is validated by Cloud Run before the request
// reaches this handler.
type InternalDispatchHandler struct {
	planner   planner
	processor processor
	logger    *slog.Logger
}

type processor interface {
	Process(ctx context.Context, req ExecutePlanRequest) error
}

type planner interface {
	GenerateExecutionPlan(ctx context.Context, req ExecutePlanRequest) (*domain.JobExecutionPlan, error)
}

type internalDispatchRequest struct {
	Procedure string                    `json:"procedure"`
	Payload   domain.ExecutePlanRequest `json:"payload"`
}

func NewInternalDispatchHandler(p processor, pl planner, logger *slog.Logger) *InternalDispatchHandler {
	return &InternalDispatchHandler{planner: pl, processor: p, logger: logger}
}

// ServeHTTP handles POST /internal/dispatch-job. Errors that should NOT be
// retried (validation, unknown procedure, rejected plan) return 4xx;
// everything else returns 500 so Cloud Tasks backs off and retries. A
// successful no-op (e.g. the job is already terminal) returns 200.
func (h *InternalDispatchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req internalDispatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("worker.internal_dispatch.decode_failed", "error", err.Error())
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := req.Payload.Validate(); err != nil {
		h.logger.Warn("worker.internal_dispatch.invalid_payload", "error", err.Error(), "job_id", req.Payload.JobID)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.logger.Info("worker.internal_dispatch.received",
		"procedure", req.Procedure,
		"job_id", req.Payload.JobID,
		"document_id", req.Payload.DocumentID,
	)

	ctx := r.Context()
	switch req.Procedure {
	case "GenerateExecutionPlan":
		if _, err := h.planner.GenerateExecutionPlan(ctx, req.Payload); err != nil {
			h.logger.Error("worker.internal_dispatch.generate_failed", "error", err.Error(), "job_id", req.Payload.JobID)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case "ExecuteApprovedPlan":
		if err := h.processor.Process(ctx, req.Payload); err != nil {
			if errors.Is(err, ErrApprovalRequired) || errors.Is(err, ErrPlanRejected) {
				// A rejected / pending-approval plan is a terminal decision,
				// not a transient failure, so do not let Cloud Tasks retry.
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			h.logger.Error("worker.internal_dispatch.execute_failed", "error", err.Error(), "job_id", req.Payload.JobID)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	default:
		h.logger.Warn("worker.internal_dispatch.unknown_procedure", "procedure", req.Procedure)
		http.Error(w, "unknown procedure", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}
