package repository

import (
	"context"
	"time"

	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
	joblog "github.com/synthify/backend/internal/platform/job/log"
)

type DocumentUploadTarget struct {
	URL         string
	Method      string
	ContentType string
	ExpiresAt   time.Time
}

type DocumentUploadURLIssuer interface {
	IssueDocumentUploadURL(ctx context.Context, workspaceID, objectName, contentType string, fileSize int64) (DocumentUploadTarget, error)
}

type DocumentSourceURLBuilder func(workspaceID, documentID string) string

// Repositories は単一 unit-of-work 内で利用可能な全リポジトリの集合体。
// postgres / mock Store の双方が実装する。
type Repositories interface {
	AccountRepository
	WorkspaceRepository
	DocumentRepository
	TreeRepository
	ItemRepository
	UsageRepository
	CheckpointRepository
	DynamicToolRepository
	JobLogWriter
}

// Transactor は service / worker 層から tx 境界を制御するためのインターフェース。
type Transactor interface {
	WithTx(ctx context.Context, fn func(Repositories) error) error
}

type AccountRepository interface {
	GetOrCreateAccount(ctx context.Context, userID string) (*domain.Account, error)
	GetAccount(ctx context.Context, accountID string) (*domain.Account, error)
	IsAccountAccessible(ctx context.Context, accountID, userID string) (ok bool, err error)
	SetAccountStripeCustomerID(ctx context.Context, accountID, stripeCustomerID string) error
	ListStripeLinkedAccounts(ctx context.Context, limit int) ([]*domain.Account, error)
	ApplyBillingPlan(ctx context.Context, accountID, stripeCustomerID, stripeSubscriptionID string, plan domain.BillingPlan) error
	ApplyBillingPlanByStripeCustomerID(ctx context.Context, stripeCustomerID, stripeSubscriptionID string, plan domain.BillingPlan) error
	RecordBillingWebhookEvent(ctx context.Context, event *domain.ProviderWebhookEvent) (bool, error)
	MarkBillingWebhookEventProcessed(ctx context.Context, provider, eventID, status, errorMessage string) error
	ApplyBillingEvent(ctx context.Context, event *domain.ProviderWebhookEvent) error
}

type WorkspaceRepository interface {
	ListWorkspacesByUser(ctx context.Context, userID string) ([]*domain.Workspace, error)
	GetWorkspace(ctx context.Context, id string) (*domain.Workspace, error)
	// IsWorkspaceAccessible は DB エラーと「アクセス不可」を区別して返す。
	IsWorkspaceAccessible(ctx context.Context, wsID, userID string) (ok bool, err error)
	// CreateWorkspace は workspaces + tree root item を 1 ペアで作成する。
	// atomic 性が必要なら呼び出し側を Transactor.WithTx で包むこと。
	CreateWorkspace(ctx context.Context, accountID, name string) (*domain.Workspace, error)
	UpdateWorkspaceName(ctx context.Context, workspaceID, name string) (*domain.Workspace, error)
	DeleteWorkspace(ctx context.Context, workspaceID string) error
}

type DocumentRepository interface {
	ListDocuments(ctx context.Context, wsID string) ([]*domain.Document, error)
	GetDocument(ctx context.Context, id string) (*domain.Document, error)
	GetDocumentChunks(ctx context.Context, documentID string) ([]*domain.DocumentChunk, error)
	GetJobPlanningSignals(ctx context.Context, documentID, workspaceID, treeID string) (*domain.JobPlanningSignals, error)
	CreateDocument(ctx context.Context, wsID, uploadedBy, filename, mimeType string, fileSize int64) (*domain.Document, DocumentUploadTarget, error)
	ConfirmDocumentUpload(ctx context.Context, documentID string, actualSize int64) error
	ExpireUploadReservations(ctx context.Context, now time.Time) (int64, error)
	CreateDocumentFile(ctx context.Context, docID, path, mimeType string, fileSize int64) (*domain.DocumentFile, error)
	ListDocumentFiles(ctx context.Context, docID string) ([]*domain.DocumentFile, error)
	GetDocumentFileByPath(ctx context.Context, docID, path string) (*domain.DocumentFile, error)
	GetLatestProcessingJob(ctx context.Context, docID string) (*domain.DocumentProcessingJob, error)
	GetProcessingJob(ctx context.Context, jobID string) (*domain.DocumentProcessingJob, error)
	GetJobCapability(ctx context.Context, jobID string) (*domain.JobCapability, error)
	GetJobExecutionPlan(ctx context.Context, jobID string) (*domain.JobExecutionPlan, error)
	UpsertJobExecutionPlan(ctx context.Context, jobID string, plan *domain.JobExecutionPlan) error
	UpsertJobEvaluation(ctx context.Context, jobID string, result *domain.JobEvaluationResult) error
	EvaluateJob(ctx context.Context, jobID string) (*domain.JobEvaluationResult, error)
	ListJobApprovalRequests(ctx context.Context, jobID string) ([]*domain.JobApprovalRequest, error)
	RequestJobApproval(ctx context.Context, jobID, requestedBy, reason string) (*domain.JobApprovalRequest, error)
	ApproveJobApproval(ctx context.Context, jobID, approvalID, reviewedBy string) error
	RejectJobApproval(ctx context.Context, jobID, approvalID, reviewedBy, reason string) error
	SearchRelatedChunksByVector(ctx context.Context, workspaceID string, embedding []float32, limit int) ([]*domain.DocumentChunk, error)
	LogToolCall(ctx context.Context, jobID, toolName, inputJSON, outputJSON string, durationMs int64) error
	// CreateProcessingJob は job_capabilities と processing_jobs を 1 ペアで作成する。
	// 内部で tx を張らないため、複数 sqlc 呼び出しの atomic 性が必要な場合は
	// 呼び出し側を Transactor.WithTx で包むこと。
	CreateProcessingJob(ctx context.Context, docID, workspaceID, requestedBy string, jobType appv1.JobType) (*domain.DocumentProcessingJob, error)
	MarkProcessingJobRunning(ctx context.Context, jobID string) error
	UpdateProcessingJobStage(ctx context.Context, jobID, stage string) error
	FailProcessingJob(ctx context.Context, jobID, errorMessage string) error
	RequeueProcessingJobForRetry(ctx context.Context, jobID string) error
	CompleteProcessingJob(ctx context.Context, jobID string) error
	SaveDocumentChunks(ctx context.Context, documentID string, chunks []*domain.DocumentChunk) error
	ListJobMutationLogs(ctx context.Context, jobID string) ([]*domain.JobMutationLog, error)
	ListAllJobs(ctx context.Context) ([]*domain.DocumentProcessingJob, error)
	ListJobLogs(ctx context.Context, jobID string, pageToken string, limit int) ([]*domain.JobLog, string, error)
	SearchJobLogs(ctx context.Context, filter domain.JobLogSearchFilter) ([]*domain.JobLog, string, error)
	ListRelatedJobLogs(ctx context.Context, scope domain.RelatedLogScope, workspaceID, documentID, jobID string, pageToken string, limit int) ([]*domain.JobLogGroup, string, error)
}

type TreeRepository interface {
	GetTreeByWorkspace(ctx context.Context, wsID string) ([]*domain.Item, error)
	GetWorkspaceRootItemID(ctx context.Context, wsID string) (string, error)
	// GetDocumentRootItemID returns the document_root tree_item id linked
	// to the given documentID via document_tree_links. ErrNotFound when no
	// processing job has persisted items for the document yet.
	GetDocumentRootItemID(ctx context.Context, documentID string) (string, error)
	FindPaths(ctx context.Context, wsID, sourceItemID, targetItemID string, maxDepth, limit int) ([]*domain.Item, []domain.TreePath, error)
	GetSubtree(ctx context.Context, rootItemID string, maxDepth int) ([]*domain.SubtreeItem, error)
}

type ItemRepository interface {
	GetItem(ctx context.Context, itemID string) (*domain.Item, error)
	CreateItem(ctx context.Context, workspaceID, label, description, parentID, createdBy string) (*domain.Item, error)
	// CreateStructuredItemWithCapability は capability の検証 + item 挿入 +
	// mutation log を行う。capability 違反 (op 不許可、ws 不一致、上限超過、expired) は
	// domain.ErrForbidden を返す。atomic 性が必要なら Transactor.WithTx で包むこと。
	CreateStructuredItemWithCapability(ctx context.Context, capability *domain.JobCapability, jobID, documentID, workspaceID, label string, level int, description, summaryHTML, overrideCSS, createdBy, parentID string, sourceChunkIDs []string) (*domain.Item, error)
	// CreateDocumentRootItemWithCapability inserts the document_root item
	// for a given document, atomically registering the 1:1 link in
	// document_tree_links. Use this exactly once per document; subsequent
	// items in the same document should be created via
	// CreateStructuredItemWithCapability with parent_id set to the returned
	// item's ID.
	CreateDocumentRootItemWithCapability(ctx context.Context, capability *domain.JobCapability, jobID, documentID, workspaceID, label, description, workspaceRootItemID string) (*domain.Item, error)
	UpsertItemSource(ctx context.Context, itemID, documentID, fileID, chunkID, sourceText string, confidence float64) error
	UpdateItemSummaryHTMLWithCapability(ctx context.Context, capability *domain.JobCapability, jobID, itemID, summaryHTML string) error
	ApproveAlias(ctx context.Context, wsID, canonicalItemID, aliasItemID string) error
	RejectAlias(ctx context.Context, wsID, canonicalItemID, aliasItemID string) error
}

type UsageRepository interface {
	GetModelPricing(ctx context.Context, model string) (*domain.ModelPricing, error)
	RecordUsageAccounting(ctx context.Context, ev *domain.UsageEvent, date string) (string, bool, error)
	ListUsageByModel(ctx context.Context, accountID, periodStart, periodEnd string) ([]domain.ModelUsage, string, error)
	ListDailyUsage(ctx context.Context, accountID, periodStart, periodEnd, currency string) ([]domain.DailyUsage, error)
	UpdateAccountBudgetLimit(ctx context.Context, accountID string, limitMinor int64) error
	ListInvoices(ctx context.Context, accountID string, limit int) (*domain.InvoiceList, error)
	ListPaymentMethods(ctx context.Context, accountID string) ([]*domain.PaymentMethod, error)

	// Credits
	GrantCredit(ctx context.Context, grant *domain.CreditGrant) error
	GetCreditBalance(ctx context.Context, accountID string) (int64, error)
	ListCreditGrants(ctx context.Context, accountID string, limit int) ([]*domain.CreditGrant, error)
}

type JobLogWriter interface {
	LogJobEvent(ctx context.Context, e joblog.Event) error
}

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
