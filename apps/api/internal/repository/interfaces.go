package repository

import (
	"context"
	"time"

	"github.com/synthify/backend/internal/domain"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
	joblog "github.com/synthify/backend/internal/platform/job/log"
)

// DocumentUploadTarget contains target details and metadata for direct document uploads to storage.
type DocumentUploadTarget struct {
	// URL is the signed upload URL.
	URL string
	// Method is the HTTP method to use for the upload (PUT or POST).
	Method string
	// ContentType is the exact MIME type bound into the signature.
	ContentType string
	// ExpiresAt is the timestamp when the upload target expires.
	ExpiresAt time.Time
}

// DocumentUploadURLIssuer issues short-lived direct-to-storage upload URLs for documents.
type DocumentUploadURLIssuer interface {
	// IssueDocumentUploadURL issues a signed upload URL to storage. fileSize is bound
	// into the signature via the Content-Length header; it is always validated
	// to be positive by the caller (CreateDocument) before reaching here.
	IssueDocumentUploadURL(ctx context.Context, workspaceID, objectName, contentType string, fileSize int64) (DocumentUploadTarget, error)
}

// SignedURL represents a short-lived signed GET URL along with its expiration timestamp.
type SignedURL struct {
	// URL is the pre-signed URL.
	URL string
	// ExpiresAt is the time at which the URL ceases to be valid.
	ExpiresAt time.Time
}

// DocumentImageURLIssuer issues a short-lived signed GET URL for a document's
// uploaded original, used to render images embedded in generated HTML by their
// data-file-id marker. The url is minted fresh per view and expires, so it is
// never persisted into the saved HTML.
type DocumentImageURLIssuer interface {
	// IssueDocumentImageURL generates a short-lived signed GET URL for downloading or rendering document images.
	IssueDocumentImageURL(ctx context.Context, workspaceID, documentID, filename string) (SignedURL, error)
}

// DocumentObjectStore handles storage cleanup for document objects uploaded to cloud storage.
// Used to delete orphaned blobs upon upload cancellation, expiration, or reservation mismatch.
type DocumentObjectStore interface {
	// DeleteDocumentObject removes the specified document object and its subtree from storage.
	DeleteDocumentObject(ctx context.Context, workspaceID, objectName string) error
}

// Repositories aggregates all repository interfaces accessible within a single unit of work.
// By embedding distinct repository interfaces, it enables cross-repository atomic operations
// within a single transaction scope.
type Repositories interface {
	AccountRepository
	UserRepository
	WorkspaceRepository
	DocumentRepository
	DocumentChunkRepository
	JobRepository
	JobApprovalRepository
	JobLogRepository
	TreeRepository
	ItemRepository
	UsageRepository
	CheckpointRepository
	DynamicToolRepository
	WorkspaceChatRepository
}

// Transactor controls transactional boundaries from application service workflows.
// If the callback returns nil, the transaction commits; if it returns an error, it rolls back.
type Transactor interface {
	// WithTx executes the supplied function within an isolated database transaction.
	WithTx(ctx context.Context, fn func(Repositories) error) error
}

// DocumentSourceURLBuilder constructs the canonical retrieval or delivery URL for a document source.
type DocumentSourceURLBuilder func(workspaceID, documentID string) string

// AccountRepository defines operations for managing accounts, membership, billing plans, and webhooks.
type AccountRepository interface {
	GetOrCreateAccount(ctx context.Context, userID string) (*domain.Account, error)
	GetAccount(ctx context.Context, accountID string) (*domain.Account, error)
	GetAccountByUser(ctx context.Context, userID string) (*domain.Account, error)
	CreateAccount(ctx context.Context, userID string) (*domain.Account, error)
	// IsAccountAccessible reports whether the user has access to the account, distinguishing DB error from access denied.
	IsAccountAccessible(ctx context.Context, accountID, userID string) (ok bool, err error)
	SetAccountStripeCustomerID(ctx context.Context, accountID, stripeCustomerID string) error
	ListStripeLinkedAccounts(ctx context.Context, limit int) ([]*domain.Account, error)
	ApplyBillingPlan(ctx context.Context, accountID, stripeCustomerID, stripeSubscriptionID string, plan domain.BillingPlan) error
	ApplyBillingPlanByStripeCustomerID(ctx context.Context, stripeCustomerID, stripeSubscriptionID string, plan domain.BillingPlan) error
	RecordBillingWebhookEvent(ctx context.Context, event *domain.ProviderWebhookEvent) (bool, error)
	MarkBillingWebhookEventProcessed(ctx context.Context, provider, eventID, status, errorMessage string) error
	ApplyBillingEvent(ctx context.Context, event *domain.ProviderWebhookEvent) error
}

// UserRepository defines operations for reading and writing user accounts.
type UserRepository interface {
	GetUser(ctx context.Context, userID string) (*domain.User, error)
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
	UpsertUser(ctx context.Context, user *domain.User) (*domain.User, error)
}

// WorkspaceRepository defines operations for workspaces, collaborator memberships, and public share links.
type WorkspaceRepository interface {
	ListWorkspacesByUser(ctx context.Context, userID string) ([]*domain.Workspace, error)
	GetWorkspace(ctx context.Context, id string) (*domain.Workspace, error)
	// IsWorkspaceAccessible checks if the user has access to the workspace. If err != nil, ok has no meaning.
	IsWorkspaceAccessible(ctx context.Context, wsID, userID string) (ok bool, err error)
	// GetWorkspaceRole returns the user's role in the workspace. Account owners map to owner role.
	// If the user has no membership, returns empty string.
	GetWorkspaceRole(ctx context.Context, wsID, userID string) (domain.WorkspaceRole, error)
	// CreateWorkspace creates a workspace and root tree item pair.
	CreateWorkspace(ctx context.Context, accountID, name string) (*domain.Workspace, error)
	UpdateWorkspaceName(ctx context.Context, workspaceID, name string) (*domain.Workspace, error)
	DeleteWorkspace(ctx context.Context, workspaceID string) error
	// UpsertWorkspaceMember invites or updates a workspace collaborator's role.
	UpsertWorkspaceMember(ctx context.Context, wsID, userID string, role domain.WorkspaceRole, invitedBy string) error
	// ListWorkspaceMembers returns all invited workspace members including emails.
	ListWorkspaceMembers(ctx context.Context, wsID string) ([]*domain.WorkspaceMember, error)
	// RemoveWorkspaceMember deletes an invited workspace member.
	RemoveWorkspaceMember(ctx context.Context, wsID, userID string) error
	// CreateShareLink creates a public share link for unauthenticated or collaborator access.
	CreateShareLink(ctx context.Context, link *domain.ShareLink) error
	// ListShareLinks returns all share links (including revoked/expired) for the workspace ordered by recency.
	ListShareLinks(ctx context.Context, wsID string) ([]*domain.ShareLink, error)
	// RevokeShareLink invalidates an active share link.
	RevokeShareLink(ctx context.Context, wsID, token string, now time.Time) error
	// ResolveShareLink returns a valid (unrevoked and unexpired) share link by its token.
	ResolveShareLink(ctx context.Context, token string, now time.Time) (*domain.ShareLink, error)
}

// DocumentRepository handles document and file metadata persistence and storage reservations.
type DocumentRepository interface {
	ListDocuments(ctx context.Context, wsID string) ([]*domain.Document, error)
	GetDocument(ctx context.Context, id string) (*domain.Document, error)
	CreateDocument(ctx context.Context, wsID, uploadedBy, filename, mimeType string, fileSize int64) (*domain.Document, DocumentUploadTarget, error)
	ConfirmDocumentUpload(ctx context.Context, documentID string, actualSize int64) error
	ExpireUploadReservations(ctx context.Context, now time.Time) ([]domain.ExpiredReservation, error)
	CreateDocumentFile(ctx context.Context, docID, path, mimeType string, fileSize int64) (*domain.DocumentFile, error)
	ListDocumentFiles(ctx context.Context, docID string) ([]*domain.DocumentFile, error)
	GetDocumentFileByPath(ctx context.Context, docID, path string) (*domain.DocumentFile, error)
	GetDocumentFileForDelivery(ctx context.Context, fileID string) (*domain.DocumentFileLocation, error)
}

// DocumentChunkRepository manages chunked document content and vector similarity search.
type DocumentChunkRepository interface {
	GetDocumentChunks(ctx context.Context, documentID string) ([]*domain.DocumentChunk, error)
	SaveDocumentChunks(ctx context.Context, documentID string, chunks []*domain.DocumentChunk) error
	SearchRelatedChunksByVector(ctx context.Context, workspaceID string, embedding []float32, limit int) ([]*domain.DocumentChunk, error)
}

// JobRepository manages the lifecycle, execution plans, and evaluations for document processing jobs.
type JobRepository interface {
	GetLatestProcessingJob(ctx context.Context, docID string) (*domain.DocumentProcessingJob, error)
	GetProcessingJob(ctx context.Context, jobID string) (*domain.DocumentProcessingJob, error)
	GetJobCapability(ctx context.Context, jobID string) (*domain.JobCapability, error)
	GetJobExecutionPlan(ctx context.Context, jobID string) (*domain.JobExecutionPlan, error)
	UpsertJobExecutionPlan(ctx context.Context, jobID string, plan *domain.JobExecutionPlan) error
	UpsertJobEvaluation(ctx context.Context, jobID string, result *domain.JobEvaluationResult) error
	EvaluateJob(ctx context.Context, jobID string) (*domain.JobEvaluationResult, error)
	// CreateProcessingJob creates a processing job and associated job capability.
	CreateProcessingJob(ctx context.Context, docID, workspaceID, requestedBy string, jobType appv1.JobType) (*domain.DocumentProcessingJob, error)
	MarkProcessingJobRunning(ctx context.Context, jobID string) error
	UpdateProcessingJobStage(ctx context.Context, jobID, stage string) error
	FailProcessingJob(ctx context.Context, jobID, errorMessage string) error
	SetProcessingJobRetryCount(ctx context.Context, jobID string, retryCount int) error
	CompleteProcessingJob(ctx context.Context, jobID string) error
	ListAllJobs(ctx context.Context) ([]*domain.DocumentProcessingJob, error)
	GetJobPlanningSignals(ctx context.Context, documentID, workspaceID, treeID string) (*domain.JobPlanningSignals, error)
}

// JobApprovalRepository manages human-in-the-loop review and approval workflows for execution plans.
type JobApprovalRepository interface {
	ListJobApprovalRequests(ctx context.Context, jobID string) ([]*domain.JobApprovalRequest, error)
	RequestJobApproval(ctx context.Context, jobID, requestedBy, reason string) (*domain.JobApprovalRequest, error)
	ApproveJobApproval(ctx context.Context, jobID, approvalID, reviewedBy string) error
	RejectJobApproval(ctx context.Context, jobID, approvalID, reviewedBy, reason string) error
}

// JobLogRepository records execution logs, mutation records, and tool invocation audits.
type JobLogRepository interface {
	LogToolCall(ctx context.Context, jobID, toolName, inputJSON, outputJSON string, durationMs int64) error
	ListJobMutationLogs(ctx context.Context, jobID string) ([]*domain.JobMutationLog, error)
	ListJobLogs(ctx context.Context, jobID string, pageToken string, limit int) ([]*domain.JobLog, string, error)
	SearchJobLogs(ctx context.Context, filter domain.JobLogSearchFilter) ([]*domain.JobLog, string, error)
	ListRelatedJobLogs(ctx context.Context, scope domain.RelatedLogScope, workspaceID, documentID, jobID string, pageToken string, limit int) ([]*domain.JobLogGroup, string, error)
}

// TreeRepository queries knowledge tree hierarchy, paths, and subtrees.
type TreeRepository interface {
	GetTree(ctx context.Context, wsID string) (*domain.Tree, error)
	GetTreeByWorkspace(ctx context.Context, wsID string) ([]*domain.Item, error)
	FindPaths(ctx context.Context, wsID, sourceItemID, targetItemID string, maxDepth, limit int) ([]*domain.Item, []domain.TreePath, error)
	GetSubtree(ctx context.Context, rootItemID string, maxDepth int) ([]*domain.SubtreeItem, error)
}

// ItemRepository manages knowledge tree items, provenance sources, and alias relations.
type ItemRepository interface {
	GetItem(ctx context.Context, itemID string) (*domain.Item, error)
	CreateItem(ctx context.Context, workspaceID, label, description, parentID, createdBy string) (*domain.Item, error)
	// CreateStructuredItemWithCapability validates job capabilities and inserts structured tree items with mutation tracking.
	CreateStructuredItemWithCapability(ctx context.Context, capability *domain.JobCapability, jobID, documentID, workspaceID, label string, level int, description, summaryHTML, overrideCSS, createdBy, parentID string, sourceChunkIDs []string) (*domain.Item, error)
	UpsertItemSource(ctx context.Context, itemID, documentID, fileID, chunkID, sourceText string, confidence float64) error
	UpsertItemSourceRef(ctx context.Context, source *domain.ItemSourceRef) error
	ListItemSources(ctx context.Context, itemID string) ([]*domain.ItemSource, error)
	ListItemSourceRefs(ctx context.Context, itemID string) ([]*domain.ItemSourceRef, error)
	UpdateItemSummaryHTMLWithCapability(ctx context.Context, capability *domain.JobCapability, jobID, itemID, summaryHTML string) error
	ApproveAlias(ctx context.Context, wsID, canonicalItemID, aliasItemID string) error
	RejectAlias(ctx context.Context, wsID, canonicalItemID, aliasItemID string) error
}

// WorkspaceChatRepository handles conversation threads, messages, and citation candidate retrieval.
// Chat is strictly read-only regarding document and tree structures; no mutation methods belong here.
type WorkspaceChatRepository interface {
	CreateChatConversation(ctx context.Context, workspaceID, createdBy, title string) (*domain.ChatConversation, error)
	GetChatConversation(ctx context.Context, conversationID string) (*domain.ChatConversation, error)
	ListChatConversations(ctx context.Context, workspaceID string, limit int) ([]*domain.ChatConversation, error)
	TouchChatConversation(ctx context.Context, conversationID string) error

	// CreateChatMessage persists a chat message along with its verified citation sources.
	CreateChatMessage(ctx context.Context, msg *domain.ChatMessage, retrievalSnapshot []byte) (*domain.ChatMessage, error)
	ListChatMessages(ctx context.Context, conversationID string, limit int) ([]*domain.ChatMessage, error)
	// ListRecentChatMessages returns the latest N messages in chronological order for prompt context.
	ListRecentChatMessages(ctx context.Context, conversationID string, limit int) ([]*domain.ChatMessage, error)

	// SearchChatSourceCandidates retrieves citation candidate chunks from succeeded documents matching search terms.
	SearchChatSourceCandidates(ctx context.Context, workspaceID string, terms []string, limit int) ([]domain.ChatSourceCandidate, error)
	// SearchChatSourceItems retrieves citation candidate items from the knowledge tree matching search terms.
	SearchChatSourceItems(ctx context.Context, workspaceID string, terms []string, limit int) ([]domain.ChatSourceCandidate, error)
	CountChatSourceDocuments(ctx context.Context, workspaceID string) (int, error)
	CountChatSourceItems(ctx context.Context, workspaceID string) (int, error)
}

// UsageRepository manages LLM token accounting, pricing catalog, usage-based billing, invoices, and credits.
type UsageRepository interface {
	GetModelPricing(ctx context.Context, model string) (*domain.ModelPricing, error)
	RecordUsageAccounting(ctx context.Context, ev *domain.UsageEvent, date string) (string, bool, error)
	ListUsageByModel(ctx context.Context, accountID, periodStart, periodEnd string) ([]domain.ModelUsage, string, error)
	ListDailyUsage(ctx context.Context, accountID, periodStart, periodEnd, currency string) ([]domain.DailyUsage, error)
	UpdateAccountBudgetLimit(ctx context.Context, accountID string, limitMinor int64) error
	ListInvoices(ctx context.Context, accountID string, limit int) (*domain.InvoiceList, error)
	ListPaymentMethods(ctx context.Context, accountID string) ([]*domain.PaymentMethod, error)

	// UpsertInvoice persists cached Stripe invoice details.
	UpsertInvoice(ctx context.Context, stripeCustomerID string, inv *domain.ProviderInvoice) error
	// UpsertPaymentMethod persists cached customer payment method details.
	UpsertPaymentMethod(ctx context.Context, stripeCustomerID string, pm *domain.ProviderPaymentMethod) error
	// DeletePaymentMethod removes a payment method from the cache upon detachment.
	DeletePaymentMethod(ctx context.Context, stripePaymentMethodID string) error
	// SetDefaultPaymentMethod records the active default payment method identifier.
	SetDefaultPaymentMethod(ctx context.Context, stripeCustomerID, defaultPaymentMethodID string) error

	// Credits
	GrantCredit(ctx context.Context, grant *domain.CreditGrant) error
	GetCreditBalance(ctx context.Context, accountID string) (int64, error)
	ListCreditGrants(ctx context.Context, accountID string, limit int) ([]*domain.CreditGrant, error)
}

// JobLogWriter writes structured runtime lifecycle events for background jobs.
type JobLogWriter interface {
	// LogJobEvent records a structured job execution event.
	LogJobEvent(ctx context.Context, e joblog.Event) error
}

// CheckpointRepository persists pipeline execution stage status and intermediate cloud storage checkpoint artifacts.
type CheckpointRepository interface {
	UpsertStageRunning(ctx context.Context, jobID, stage string) error
	MarkStageSucceeded(ctx context.Context, jobID, stage, gcsRef string) error
	MarkStageFailed(ctx context.Context, jobID, stage, errorMessage string) error
	ListStageCheckpoints(ctx context.Context, jobID string) ([]domain.JobStageCheckpoint, error)
}

// DynamicToolRepository is the persistence contract for LLM-generated
// transform tools. Recording happens inside the job; tier judgement and
// promotion run out-of-band (see docs/improvements/dynamic-tool-synthesis.md).
type DynamicToolRepository interface {
	// RecordCandidate persists a freshly generated tool with status=candidate.
	// Version is assigned by the store as max(existing)+1 under the unique
	// constraint (scope, origin_workspace_id, name, version); the returned tool
	// carries the assigned ToolID and Version.
	RecordCandidate(ctx context.Context, tool *domain.DynamicTool) (*domain.DynamicTool, error)

	// ListCandidates returns candidates awaiting tier judgement / promotion,
	// oldest first, capped by limit. Used by the out-of-band promotion path.
	ListCandidates(ctx context.Context, limit int) ([]*domain.DynamicTool, error)

	// PromoteCandidate transitions candidate -> active (or -> held for tier_3).
	// Implementations must lock the row (FOR UPDATE) to prevent double promotion.
	PromoteCandidate(ctx context.Context, toolID string, status domain.ToolStatus, promotedAt time.Time) error

	// PromoteToGlobal flips an active workspace-scoped tool to global scope.
	// This is an explicit additional promotion, independent of tier.
	PromoteToGlobal(ctx context.Context, toolID, reviewedBy string) error

	// SetStatus is the kill switch / review-decision write path
	// (e.g. -> disabled, -> rejected, -> held).
	SetStatus(ctx context.Context, toolID string, status domain.ToolStatus) error

	// ResolveActiveTools returns tools the registry may load for a job in the
	// given workspace: that workspace's active tools plus global active tools.
	// Default-deny — never returns other tenants' workspace-scoped tools.
	ResolveActiveTools(ctx context.Context, workspaceID string) ([]*domain.DynamicTool, error)

	// IncrementUseCount bumps usage (ephemeral + promoted) for lifecycle/GC.
	IncrementUseCount(ctx context.Context, toolID string) error
}
