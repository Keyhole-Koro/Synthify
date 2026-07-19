package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/synthify/backend/apps/api/internal/application"
)

type DevSeedUsecase interface {
	SeedWorkspace(ctx context.Context, userID string) (*application.DevSeedResult, error)
}

type DevSeedHTTPHandler struct {
	service DevSeedUsecase
	enabled bool
}

func NewDevSeedHTTPHandler(svc DevSeedUsecase, enabled bool) *DevSeedHTTPHandler {
	return &DevSeedHTTPHandler{service: svc, enabled: enabled}
}

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

	result, err := h.application.SeedWorkspace(r.Context(), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
