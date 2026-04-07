package handler

import (
	"net/http"

	"github.com/synthify/backend/internal/repository/mock"
)

type DocumentHandler struct {
	store *mock.Store
}

func NewDocumentHandler(store *mock.Store) *DocumentHandler {
	return &DocumentHandler{store: store}
}

func (h *DocumentHandler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WorkspaceID string `json:"workspace_id"`
	}
	if err := decodeBody(r, &req); err != nil || req.WorkspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	docs := h.store.ListDocuments(req.WorkspaceID)
	writeJSON(w, map[string]any{"documents": docs})
}

func (h *DocumentHandler) GetDocument(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DocumentID string `json:"document_id"`
	}
	if err := decodeBody(r, &req); err != nil || req.DocumentID == "" {
		writeError(w, http.StatusBadRequest, "document_id is required")
		return
	}
	doc, ok := h.store.GetDocument(req.DocumentID)
	if !ok {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}
	writeJSON(w, map[string]any{"document": doc})
}

func (h *DocumentHandler) CreateDocument(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WorkspaceID string `json:"workspace_id"`
		Filename    string `json:"filename"`
		MimeType    string `json:"mime_type"`
		FileSize    int64  `json:"file_size"`
	}
	if err := decodeBody(r, &req); err != nil || req.WorkspaceID == "" || req.Filename == "" {
		writeError(w, http.StatusBadRequest, "workspace_id and filename are required")
		return
	}
	doc, uploadURL := h.store.CreateDocument(req.WorkspaceID, req.Filename, req.MimeType, req.FileSize)
	writeJSON(w, map[string]any{
		"document":             doc,
		"upload_url":           uploadURL,
		"upload_method":        "PUT",
		"upload_content_type":  req.MimeType,
	})
}

func (h *DocumentHandler) StartProcessing(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DocumentID      string `json:"document_id"`
		ForceReprocess  bool   `json:"force_reprocess"`
		ExtractionDepth string `json:"extraction_depth"`
	}
	if err := decodeBody(r, &req); err != nil || req.DocumentID == "" {
		writeError(w, http.StatusBadRequest, "document_id is required")
		return
	}
	doc, ok := h.store.StartProcessing(req.DocumentID, req.ForceReprocess, req.ExtractionDepth)
	if !ok {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}
	writeJSON(w, map[string]any{
		"document_id": doc.DocumentID,
		"status":      doc.Status,
		"job_id":      "job_" + doc.DocumentID,
	})
}

func (h *DocumentHandler) ResumeProcessing(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DocumentID string `json:"document_id"`
	}
	if err := decodeBody(r, &req); err != nil || req.DocumentID == "" {
		writeError(w, http.StatusBadRequest, "document_id is required")
		return
	}
	doc, ok := h.store.GetDocument(req.DocumentID)
	if !ok {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}
	writeJSON(w, map[string]any{
		"document_id": doc.DocumentID,
		"status":      "processing",
		"job_id":      "job_resume_" + doc.DocumentID,
	})
}
