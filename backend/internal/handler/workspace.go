package handler

import (
	"net/http"

	"github.com/synthify/backend/internal/repository/mock"
)

type WorkspaceHandler struct {
	store *mock.Store
}

func NewWorkspaceHandler(store *mock.Store) *WorkspaceHandler {
	return &WorkspaceHandler{store: store}
}

func (h *WorkspaceHandler) ListWorkspaces(w http.ResponseWriter, r *http.Request) {
	workspaces := h.store.ListWorkspaces()
	writeJSON(w, map[string]any{"workspaces": workspaces})
}

func (h *WorkspaceHandler) GetWorkspace(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WorkspaceID string `json:"workspace_id"`
	}
	if err := decodeBody(r, &req); err != nil || req.WorkspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	ws, members, ok := h.store.GetWorkspace(req.WorkspaceID)
	if !ok {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	writeJSON(w, map[string]any{"workspace": ws, "members": members})
}

func (h *WorkspaceHandler) CreateWorkspace(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := decodeBody(r, &req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	ws := h.store.CreateWorkspace(req.Name)
	writeJSON(w, map[string]any{"workspace": ws})
}

func (h *WorkspaceHandler) InviteMember(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WorkspaceID string `json:"workspace_id"`
		Email       string `json:"email"`
		Role        string `json:"role"`
		IsDev       bool   `json:"is_dev"`
	}
	if err := decodeBody(r, &req); err != nil || req.WorkspaceID == "" || req.Email == "" {
		writeError(w, http.StatusBadRequest, "workspace_id and email are required")
		return
	}
	if req.Role == "" {
		req.Role = "viewer"
	}
	member, ok := h.store.InviteMember(req.WorkspaceID, req.Email, req.Role, req.IsDev)
	if !ok {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	writeJSON(w, map[string]any{"member": member})
}
