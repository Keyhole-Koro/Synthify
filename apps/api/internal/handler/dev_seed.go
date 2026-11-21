package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/synthify/backend/apps/api/internal/application"
)

// DevSeedUsecase defines the use-case behavior for development workspace seeding.
type DevSeedUsecase interface {
	SeedWorkspace(ctx context.Context, userID string) (*application.DevSeedResult, error)
}

// DevSeedHTTPHandler exposes an HTTP POST endpoint for initializing development test workspaces.
type DevSeedHTTPHandler struct {
	service DevSeedUsecase
	enabled bool
	logger  *slog.Logger
}

// NewDevSeedHTTPHandler creates a new HTTP handler instance for development seeding.
func NewDevSeedHTTPHandler(svc DevSeedUsecase, enabled bool, logger *slog.Logger) *DevSeedHTTPHandler {
	return &DevSeedHTTPHandler{service: svc, enabled: enabled, logger: logger}
}

// ServeHTTP handles HTTP POST requests to seed the caller's workspace with demo items and documents.
func (h *DevSeedHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.enabled {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := requireUserID(r.Context())
	if err != nil {
		http.Error(w, "missing authenticated user", http.StatusUnauthorized)
		return
	}

	result, err := h.service.SeedWorkspace(r.Context(), userID)
	if err != nil {
		if h.logger != nil {
			h.logger.Error("dev_seed.failed", "error", err.Error(), "user_id", userID)
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		if h.logger != nil {
			h.logger.Error("dev_seed.response_encode_failed", "error", err.Error(), "user_id", userID)
		}
	}
}
