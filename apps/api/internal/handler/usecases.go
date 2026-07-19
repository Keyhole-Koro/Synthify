package handler

import (
	"context"

	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository"
)

// BillingUsecase is the application behavior required by BillingHandler.
type BillingUsecase interface {
	GetBillingAccount(ctx context.Context, accountID, actorUserID string) (*domain.Account, error)
	CreateCheckoutSession(ctx context.Context, accountID, actorUserID string, plan domain.BillingPlan, currency domain.BillingCurrency) (*domain.BillingCheckoutSession, error)
	CreatePortalSession(ctx context.Context, accountID, actorUserID string) (*domain.BillingPortalSession, error)
	HandleWebhook(ctx context.Context, payload []byte, signature string) error
	GetUsage(ctx context.Context, accountID, actorUserID string, periodStart, periodEnd string) (*domain.UsageReport, error)
	RecordUsage(ctx context.Context, ev *domain.UsageEvent) (*domain.UsageRecordResult, error)
	UpdateBudget(ctx context.Context, accountID, actorUserID string, budgetLimit string) (string, error)
	ListInvoices(ctx context.Context, accountID, actorUserID string, limit int) (*domain.InvoiceList, error)
	ListPaymentMethods(ctx context.Context, accountID, actorUserID string) ([]*domain.PaymentMethod, error)
	GrantFreeSignupCredit(ctx context.Context, accountID string) error
	GrantCredit(ctx context.Context, actorUserID, accountID string, amountMinor int64, note string) (*domain.CreditGrant, error)
	GetCreditBalance(ctx context.Context, accountID, actorUserID string) (int64, error)
}

// DocumentUsecase is the application behavior required by DocumentHandler.
type DocumentUsecase interface {
	ListDocuments(ctx context.Context, workspaceID, userID string) ([]*domain.Document, error)
	GetDocument(ctx context.Context, documentID, userID string) (*domain.Document, error)
	CreateDocument(ctx context.Context, workspaceID, userID, filename, mimeType string, fileSize int64) (*domain.Document, repository.DocumentUploadTarget, error)
	ConfirmUpload(ctx context.Context, documentID, userID string) (*domain.Document, error)
	StartProcessing(ctx context.Context, documentID, userID string, forceReprocess bool) (*domain.DocumentProcessingJob, error)
	ResumeProcessing(ctx context.Context, documentID, userID string) (*domain.DocumentProcessingJob, error)
	IssueImageURL(ctx context.Context, fileID, userID string) (repository.SignedURL, error)
}

// WorkspaceUsecase is the application behavior required by WorkspaceHandler.
type WorkspaceUsecase interface {
	ListWorkspaces(ctx context.Context, userID string) ([]*domain.Workspace, error)
	GetWorkspace(ctx context.Context, id, userID string) (*domain.Workspace, error)
	CreateWorkspace(ctx context.Context, name, userID string) (*domain.Workspace, error)
	UpdateWorkspace(ctx context.Context, id, name, userID string) (*domain.Workspace, error)
	DeleteWorkspace(ctx context.Context, id, userID string) error
	ListMembers(ctx context.Context, workspaceID, userID string) ([]*domain.WorkspaceMember, error)
	InviteMember(ctx context.Context, workspaceID, userID, email string, role domain.WorkspaceRole) (*domain.WorkspaceMember, error)
	UpdateMemberRole(ctx context.Context, workspaceID, userID, targetUserID string, role domain.WorkspaceRole) (*domain.WorkspaceMember, error)
	RemoveMember(ctx context.Context, workspaceID, userID, targetUserID string) error
	CreateShareLink(ctx context.Context, workspaceID, userID string, role domain.WorkspaceRole, expiresAt string) (*domain.ShareLink, error)
	ListShareLinks(ctx context.Context, workspaceID, userID string) ([]*domain.ShareLink, error)
	RevokeShareLink(ctx context.Context, workspaceID, userID, token string) error
	ResolveShareLink(ctx context.Context, token string) (*domain.Workspace, domain.WorkspaceRole, error)
}
