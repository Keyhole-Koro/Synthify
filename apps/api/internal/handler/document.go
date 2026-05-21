package handler

import (
	"context"
	"errors"
	"fmt"
	"time"

	connect "connectrpc.com/connect"
	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository"
	"github.com/synthify/backend/apps/api/internal/service"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

type DocumentHandler struct {
	service         *service.DocumentService
	workspaces      repository.WorkspaceRepository
	documents       repository.DocumentRepository
	uploadURLIssuer repository.DocumentUploadURLIssuer
}

func NewDocumentHandler(
	svc *service.DocumentService,
	workspaceRepo repository.WorkspaceRepository,
	documentRepo repository.DocumentRepository,
	uploadURLIssuer repository.DocumentUploadURLIssuer,
) *DocumentHandler {
	return &DocumentHandler{
		service:         svc,
		workspaces:      workspaceRepo,
		documents:       documentRepo,
		uploadURLIssuer: uploadURLIssuer,
	}
}

func (h *DocumentHandler) ListDocuments(ctx context.Context, req *connect.Request[appv1.ListDocumentsRequest]) (*connect.Response[appv1.ListDocumentsResponse], error) {
	if req.Msg.GetWorkspaceId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id is required"))
	}
	if err := authorizeWorkspace(ctx, h.workspaces, req.Msg.GetWorkspaceId()); err != nil {
		return nil, err
	}
	docs := h.service.ListDocuments(ctx, req.Msg.GetWorkspaceId())
	res := connect.NewResponse(&appv1.ListDocumentsResponse{})
	for _, doc := range docs {
		latest, _ := h.documents.GetLatestProcessingJob(ctx, doc.DocumentID)
		res.Msg.Documents = append(res.Msg.Documents, toProtoDocument(doc, latest))
	}
	return res, nil
}

func (h *DocumentHandler) GetDocument(ctx context.Context, req *connect.Request[appv1.GetDocumentRequest]) (*connect.Response[appv1.GetDocumentResponse], error) {
	if req.Msg.GetDocumentId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("document_id is required"))
	}
	if err := authorizeDocument(ctx, h.workspaces, h.documents, req.Msg.GetDocumentId(), ""); err != nil {
		return nil, err
	}
	doc, err := h.service.GetDocument(ctx, req.Msg.GetDocumentId())
	if err != nil {
		return nil, toError(err)
	}
	latest, _ := h.documents.GetLatestProcessingJob(ctx, doc.DocumentID)
	return connect.NewResponse(&appv1.GetDocumentResponse{Document: toProtoDocument(doc, latest)}), nil
}

func (h *DocumentHandler) CreateDocument(ctx context.Context, req *connect.Request[appv1.CreateDocumentRequest]) (*connect.Response[appv1.CreateDocumentResponse], error) {
	if req.Msg.GetWorkspaceId() == "" || req.Msg.GetFilename() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id and filename are required"))
	}
	if err := authorizeWorkspace(ctx, h.workspaces, req.Msg.GetWorkspaceId()); err != nil {
		return nil, err
	}
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	doc, uploadTarget, err := h.service.CreateDocument(ctx, req.Msg.GetWorkspaceId(), user.ID, req.Msg.GetFilename(), req.Msg.GetMimeType(), req.Msg.GetFileSize())
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

func (h *DocumentHandler) GetUploadURL(ctx context.Context, req *connect.Request[appv1.GetUploadURLRequest]) (*connect.Response[appv1.GetUploadURLResponse], error) {
	if req.Msg.GetWorkspaceId() == "" || req.Msg.GetFilename() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id and filename are required"))
	}
	if err := authorizeWorkspace(ctx, h.workspaces, req.Msg.GetWorkspaceId()); err != nil {
		return nil, err
	}
	token := fmt.Sprintf("upload-%s", req.Msg.GetFilename())
	// GetUploadURL uses a special tokenized path. If that also needs to be shared,
	// extend the Generator. For now, keep the Generator as the base and wrap it as needed.
	uploadTarget, err := h.uploadURLIssuer.IssueDocumentUploadURL(ctx, req.Msg.GetWorkspaceId(), token+"/"+req.Msg.GetFilename(), req.Msg.GetMimeType())
	if err != nil {
		return nil, toError(err)
	}
	expiresAt := ""
	if !uploadTarget.ExpiresAt.IsZero() {
		expiresAt = uploadTarget.ExpiresAt.Format(time.RFC3339)
	}
	return connect.NewResponse(&appv1.GetUploadURLResponse{
		UploadUrl:   uploadTarget.URL,
		UploadToken: token,
		ExpiresAt:   expiresAt,
	}), nil
}

func (h *DocumentHandler) ConfirmUpload(ctx context.Context, req *connect.Request[appv1.ConfirmUploadRequest]) (*connect.Response[appv1.ConfirmUploadResponse], error) {
	if req.Msg.GetDocumentId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("document_id is required"))
	}
	if err := authorizeDocument(ctx, h.workspaces, h.documents, req.Msg.GetDocumentId(), ""); err != nil {
		return nil, err
	}
	doc, err := h.service.ConfirmUpload(ctx, req.Msg.GetDocumentId())
	if err != nil {
		return nil, toError(err)
	}
	latest, _ := h.documents.GetLatestProcessingJob(ctx, doc.DocumentID)
	return connect.NewResponse(&appv1.ConfirmUploadResponse{Document: toProtoDocument(doc, latest)}), nil
}

func (h *DocumentHandler) StartProcessing(ctx context.Context, req *connect.Request[appv1.StartProcessingRequest]) (*connect.Response[appv1.StartProcessingResponse], error) {
	if req.Msg.GetDocumentId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("document_id is required"))
	}
	if err := authorizeDocument(ctx, h.workspaces, h.documents, req.Msg.GetDocumentId(), ""); err != nil {
		return nil, err
	}
	doc, err := h.documents.GetDocument(ctx, req.Msg.GetDocumentId())
	if err != nil {
		return nil, toError(err)
	}
	job, err := h.service.StartProcessing(ctx, doc.WorkspaceID, req.Msg.GetDocumentId(), req.Msg.GetForceReprocess())
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

func (h *DocumentHandler) ResumeProcessing(ctx context.Context, req *connect.Request[appv1.ResumeProcessingRequest]) (*connect.Response[appv1.ResumeProcessingResponse], error) {
	if req.Msg.GetDocumentId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("document_id is required"))
	}
	if err := authorizeDocument(ctx, h.workspaces, h.documents, req.Msg.GetDocumentId(), ""); err != nil {
		return nil, err
	}
	doc, err := h.service.GetDocument(ctx, req.Msg.GetDocumentId())
	if err != nil {
		return nil, toError(err)
	}
	job, err := h.service.ResumeProcessing(ctx, doc.WorkspaceID, doc.DocumentID)
	if err != nil {
		return nil, toError(err)
	}
	return connect.NewResponse(&appv1.ResumeProcessingResponse{
		DocumentId: doc.DocumentID,
		Job: &appv1.Job{
			JobId:      job.JobID,
			DocumentId: doc.DocumentID,
			Type:       appv1.JobType_JOB_TYPE_REPROCESS_DOCUMENT,
			Status:     job.Status,
		},
	}), nil
}

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
