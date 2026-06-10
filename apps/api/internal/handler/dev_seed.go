package handler

import (
	"encoding/json"
	"net/http"

	"github.com/synthify/backend/apps/api/internal/service"
)

type DevSeedHTTPHandler struct {
	service service.DevSeedUsecase
	enabled bool
}

func NewDevSeedHTTPHandler(svc service.DevSeedUsecase, enabled bool) *DevSeedHTTPHandler {
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

	result, err := h.service.SeedWorkspace(r.Context(), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
