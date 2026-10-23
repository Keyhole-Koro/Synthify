package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	apiauth "github.com/synthify/backend/apps/api/internal/auth"
)

// MaintenanceSweeper is the periodic housekeeping the API used to run from
// in-process goroutines. Cloud Run bills always-allocated CPU for that (and with
// min-instances=0 the instance is often not even alive when the timer fires), so
// the sweeps are driven by Cloud Scheduler through this endpoint instead.
type MaintenanceSweeper interface {
	AutoResumeFailedJobs(ctx context.Context) (int, error)
	ReportStuckJobs(ctx context.Context) (int, error)
}

// MaintenanceAuthenticator authenticates the scheduler's OIDC token.
type MaintenanceAuthenticator interface {
	Authenticate(ctx context.Context, authorizationHeader string) (string, error)
}

type MaintenanceHTTPHandler struct {
	sweeper MaintenanceSweeper
	auth    MaintenanceAuthenticator
	logger  *slog.Logger
}

func NewMaintenanceHTTPHandler(sweeper MaintenanceSweeper, auth MaintenanceAuthenticator, logger *slog.Logger) *MaintenanceHTTPHandler {
	return &MaintenanceHTTPHandler{sweeper: sweeper, auth: auth, logger: logger}
}

type maintenanceSweepResult struct {
	Resumed int `json:"resumed"`
	Stuck   int `json:"stuck"`
}

func (h *MaintenanceHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	caller, err := h.auth.Authenticate(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		h.logger.Warn("maintenance.auth_failed", "error", err.Error())
		if errors.Is(err, apiauth.ErrPermissionDenied) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Both sweeps run on the request context: Cloud Run cancels it at the
	// service request timeout, which is the real budget here. The scheduler's
	// attempt deadline is set to the same value so a run is never abandoned
	// server-side while the client still waits.
	ctx := r.Context()

	resumed, err := h.sweeper.AutoResumeFailedJobs(ctx)
	if err != nil {
		h.logger.Error("job.auto_resume_scan_failed", "error", err.Error())
		http.Error(w, "auto resume sweep failed", http.StatusInternalServerError)
		return
	}

	stuck, err := h.sweeper.ReportStuckJobs(ctx)
	if err != nil {
		h.logger.Error("job.stuck_scan_failed", "error", err.Error())
		http.Error(w, "stuck job sweep failed", http.StatusInternalServerError)
		return
	}

	if resumed > 0 {
		h.logger.Info("job.auto_resume_completed", "resumed", resumed)
	}
	if stuck > 0 {
		h.logger.Warn("job.stuck_scan_found", "count", stuck)
	}
	h.logger.Info("maintenance.sweep_completed", "caller", caller, "resumed", resumed, "stuck", stuck)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(maintenanceSweepResult{Resumed: resumed, Stuck: stuck}); err != nil {
		h.logger.Error("maintenance.encode_failed", "error", err.Error())
	}
}
