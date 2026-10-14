package handler

import (
	"context"
	"errors"
	"time"

	connect "connectrpc.com/connect"
	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

// DocumentHandler implements Connect-RPC endpoints for document management and processing.
type DocumentHandler struct {
	service DocumentUsecase
	jobs    repository.JobRepository
}

// NewDocumentHandler creates a new DocumentHandler backed by the given usecase and job repository.
func NewDocumentHandler(
	svc DocumentUsecase,
	jobRepo repository.JobRepository,
) *DocumentHandler {
	return &DocumentHandler{
		service: svc,
		jobs:    jobRepo,
	}
}

// ListDocuments lists all documents within the requested workspace.
func (h *DocumentHandler) ListDocuments(ctx context.Context, req *connect.Request[appv1.ListDocumentsRequest]) (*connect.Response[appv1.ListDocumentsResponse], error) {
	if req.Msg.GetWorkspaceId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id is required"))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	docs, err := h.service.ListDocuments(ctx, req.Msg.GetWorkspaceId(), userID)
	if err != nil {
		return nil, toError(err)
	}
	res := connect.NewResponse(&appv1.ListDocumentsResponse{})
	for _, doc := range docs {
		latest, latestErr := h.jobs.GetLatestProcessingJob(ctx, doc.DocumentID)
		if latestErr != nil && !errors.Is(latestErr, domain.ErrNotFound) {
			return nil, toError(latestErr)
		}
		res.Msg.Documents = append(res.Msg.Documents, toProtoDocument(doc, latest))
	}
	return res, nil
}

// GetDocument retrieves a single document by its document ID.
func (h *DocumentHandler) GetDocument(ctx context.Context, req *connect.Request[appv1.GetDocumentRequest]) (*connect.Response[appv1.GetDocumentResponse], error) {
	if req.Msg.GetDocumentId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("document_id is required"))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	doc, err := h.service.GetDocument(ctx, req.Msg.GetDocumentId(), userID)
	if err != nil {
		return nil, toError(err)
	}
	latest, latestErr := h.jobs.GetLatestProcessingJob(ctx, doc.DocumentID)
	if latestErr != nil && !errors.Is(latestErr, domain.ErrNotFound) {
		return nil, toError(latestErr)
	}
	return connect.NewResponse(&appv1.GetDocumentResponse{Document: toProtoDocument(doc, latest)}), nil
}

// CreateDocument initializes a document record and generates a signed upload URL.
func (h *DocumentHandler) CreateDocument(ctx context.Context, req *connect.Request[appv1.CreateDocumentRequest]) (*connect.Response[appv1.CreateDocumentResponse], error) {
	if req.Msg.GetWorkspaceId() == "" || req.Msg.GetFilename() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id and filename are required"))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	doc, uploadTarget, err := h.service.CreateDocument(ctx, req.Msg.GetWorkspaceId(), userID, req.Msg.GetFilename(), req.Msg.GetMimeType(), req.Msg.GetFileSize())
	if err != nil {
		return nil, toError(err)
	}
	return connect.NewResponse(&appv1.CreateDocumentResponse{
		Document:          toProtoDocument(doc, nil),
		UploadUrl:         uploadTarget.URL,
		UploadMethod:      uploadTarget.Method,
		UploadContentType: uploadTarget.ContentType,
	}), nil
}

// GetUploadURL is an obsolete endpoint replaced by CreateDocument.
func (h *DocumentHandler) GetUploadURL(ctx context.Context, req *connect.Request[appv1.GetUploadURLRequest]) (*connect.Response[appv1.GetUploadURLResponse], error) {
	if _, err := requireUserID(ctx); err != nil {
		return nil, err
	}
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("use CreateDocument to create an authorized upload reservation"))
}

// ConfirmUpload finalizes document upload and marks it ready for processing.
func (h *DocumentHandler) ConfirmUpload(ctx context.Context, req *connect.Request[appv1.ConfirmUploadRequest]) (*connect.Response[appv1.ConfirmUploadResponse], error) {
	if req.Msg.GetDocumentId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("document_id is required"))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	doc, err := h.service.ConfirmUpload(ctx, req.Msg.GetDocumentId(), userID)
	if err != nil {
		return nil, toError(err)
	}
	latest, latestErr := h.jobs.GetLatestProcessingJob(ctx, doc.DocumentID)
	if latestErr != nil && !errors.Is(latestErr, domain.ErrNotFound) {
		return nil, toError(latestErr)
	}
	return connect.NewResponse(&appv1.ConfirmUploadResponse{Document: toProtoDocument(doc, latest)}), nil
}

// StartProcessing initiates asynchronous processing of an uploaded document.
func (h *DocumentHandler) StartProcessing(ctx context.Context, req *connect.Request[appv1.StartProcessingRequest]) (*connect.Response[appv1.StartProcessingResponse], error) {
	if req.Msg.GetDocumentId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("document_id is required"))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	job, err := h.service.StartProcessing(ctx, req.Msg.GetDocumentId(), userID, req.Msg.GetForceReprocess())
	if err != nil {
		return nil, toError(err)
	}
	return connect.NewResponse(&appv1.StartProcessingResponse{
		DocumentId: req.Msg.GetDocumentId(),
		Job: &appv1.Job{
			JobId:      job.JobID,
			DocumentId: job.DocumentID,
			Type:       appv1.JobType_JOB_TYPE_PROCESS_DOCUMENT,
			Status:     job.Status,
		},
	}), nil
}

// ResumeProcessing resumes or retries a failed document processing pipeline.
func (h *DocumentHandler) ResumeProcessing(ctx context.Context, req *connect.Request[appv1.ResumeProcessingRequest]) (*connect.Response[appv1.ResumeProcessingResponse], error) {
	if req.Msg.GetDocumentId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("document_id is required"))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	job, err := h.service.ResumeProcessing(ctx, req.Msg.GetDocumentId(), userID)
	if err != nil {
		return nil, toError(err)
	}
	return connect.NewResponse(&appv1.ResumeProcessingResponse{
		DocumentId: req.Msg.GetDocumentId(),
		Job: &appv1.Job{
			JobId:      job.JobID,
			DocumentId: job.DocumentID,
			Type:       appv1.JobType_JOB_TYPE_REPROCESS_DOCUMENT,
			Status:     job.Status,
		},
	}), nil
}

// GetImageURL retrieves a signed URL for viewing an extracted document image or page rendering.
func (h *DocumentHandler) GetImageURL(ctx context.Context, req *connect.Request[appv1.GetImageURLRequest]) (*connect.Response[appv1.GetImageURLResponse], error) {
	if req.Msg.GetFileId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("file_id is required"))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	signed, err := h.service.IssueImageURL(ctx, req.Msg.GetFileId(), userID)
	if err != nil {
		return nil, toError(err)
	}
	resp := &appv1.GetImageURLResponse{Url: signed.URL}
	if !signed.ExpiresAt.IsZero() {
		resp.ExpiresAt = signed.ExpiresAt.UTC().Format(time.RFC3339)
	}
	return connect.NewResponse(resp), nil
}

// toProtoDocument converts a domain Document and optional latest processing job into a protobuf Document.
func toProtoDocument(doc *domain.Document, latestJob *domain.DocumentProcessingJob) *appv1.Document {
	if doc == nil {
		return nil
	}
	return &appv1.Document{
		DocumentId:  doc.DocumentID,
		WorkspaceId: doc.WorkspaceID,
		UploadedBy:  doc.UploadedBy,
		Filename:    doc.Filename,
		MimeType:    doc.MimeType,
		FileSize:    doc.FileSize,
		Status:      domain.DeriveLifecycleState(latestJob),
		CreatedAt:   doc.CreatedAt,
		UpdatedAt:   doc.CreatedAt,
	}
}
