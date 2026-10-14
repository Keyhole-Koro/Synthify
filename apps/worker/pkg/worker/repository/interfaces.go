// Package repository defines data access abstractions and contracts implemented by database stores in the worker.
package repository

import (
	"context"
	"time"

	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
	joblog "github.com/synthify/backend/internal/platform/job/log"
)

// DocumentUploadTarget contains destination details and signed upload URLs for incoming files.
type DocumentUploadTarget struct {
	URL         string
	Method      string
	ContentType string
	ExpiresAt   time.Time
}

// DocumentUploadURLIssuer issues presigned URLs or local emulator endpoints for uploading document files.
type DocumentUploadURLIssuer interface {
	IssueDocumentUploadURL(ctx context.Context, workspaceID, objectName, contentType string, fileSize int64) (DocumentUploadTarget, error)
}

// DocumentSourceURLBuilder builds direct access URLs for source documents.
type DocumentSourceURLBuilder func(workspaceID, documentID string) string

// Repositories aggregates all repository surfaces available within a single transactional unit-of-work.
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

// Transactor controls atomic transaction boundaries across repository operations.
type Transactor interface {
	WithTx(ctx context.Context, fn func(Repositories) error) error
}

// AccountRepository provides CRUD operations for billing accounts and Stripe customer bindings.
type AccountRepository interface {
	GetOrCreateAccount(ctx context.Context, userID string) (*domain.Account, error)
	GetAccount(ctx context.Context, accountID string) (*domain.Account, error)
	// GetAccountByUser returns the account associated with the user, used for usage attribution.
	GetAccountByUser(ctx context.Context, userID string) (*domain.Account, error)
	IsAccountAccessible(ctx context.Context, accountID, userID string) (ok bool, err error)
	SetAccountStripeCustomerID(ctx context.Context, accountID, stripeCustomerID string) error
	ListStripeLinkedAccounts(ctx context.Context, limit int) ([]*domain.Account, error)
	ApplyBillingPlan(ctx context.Context, accountID, stripeCustomerID, stripeSubscriptionID string, plan domain.BillingPlan) error
	ApplyBillingPlanByStripeCustomerID(ctx context.Context, stripeCustomerID, stripeSubscriptionID string, plan domain.BillingPlan) error
	RecordBillingWebhookEvent(ctx context.Context, event *domain.ProviderWebhookEvent) (bool, error)
	MarkBillingWebhookEventProcessed(ctx context.Context, provider, eventID, status, errorMessage string) error
	ApplyBillingEvent(ctx context.Context, event *domain.ProviderWebhookEvent) error
}

// WorkspaceRepository handles workspace lifecycle operations and tenant verification.
type WorkspaceRepository interface {
	ListWorkspacesByUser(ctx context.Context, userID string) ([]*domain.Workspace, error)
	GetWorkspace(ctx context.Context, id string) (*domain.Workspace, error)
	// IsWorkspaceAccessible checks whether a user has access to a workspace, differentiating not-found from database errors.
	IsWorkspaceAccessible(ctx context.Context, wsID, userID string) (ok bool, err error)
	// CreateWorkspace creates a workspace and its corresponding root knowledge tree item.
	CreateWorkspace(ctx context.Context, accountID, name string) (*domain.Workspace, error)
	UpdateWorkspaceName(ctx context.Context, workspaceID, name string) (*domain.Workspace, error)
	DeleteWorkspace(ctx context.Context, workspaceID string) error
}

// DocumentRepository manages document lifecycle, file metadata, chunks, processing jobs, and execution plans.
type DocumentRepository interface {
	// ListDocuments returns all documents in a workspace ordered by creation date descending.
	ListDocuments(ctx context.Context, wsID string) ([]*domain.Document, error)
	// GetDocument retrieves a single document by its identifier.
	GetDocument(ctx context.Context, id string) (*domain.Document, error)
	// GetDocumentChunks retrieves all chunk segments belonging to a document.
	GetDocumentChunks(ctx context.Context, documentID string) ([]*domain.DocumentChunk, error)
	// GetJobPlanningSignals gathers document metadata and current knowledge tree state for orchestrator planning.
	GetJobPlanningSignals(ctx context.Context, documentID, workspaceID, treeID string) (*domain.JobPlanningSignals, error)
	// CreateDocument reserves a document record and generates an upload target.
	CreateDocument(ctx context.Context, wsID, uploadedBy, filename, mimeType string, fileSize int64) (*domain.Document, DocumentUploadTarget, error)
	// ConfirmDocumentUpload marks a document upload completed with its final verified size.
	ConfirmDocumentUpload(ctx context.Context, documentID string, actualSize int64) error
	// ExpireUploadReservations transitions pending document uploads past their deadline to expired status.
	ExpireUploadReservations(ctx context.Context, now time.Time) (int64, error)
	// CreateDocumentFile stores metadata for an individual file extracted from an archive or ingested asset.
	CreateDocumentFile(ctx context.Context, docID, path, mimeType string, fileSize int64) (*domain.DocumentFile, error)
	// ListDocumentFiles returns all extracted file records belonging to a document.
	ListDocumentFiles(ctx context.Context, docID string) ([]*domain.DocumentFile, error)
	// GetDocumentFileByPath retrieves an extracted document file by its relative path.
	GetDocumentFileByPath(ctx context.Context, docID, path string) (*domain.DocumentFile, error)
	// GetLatestProcessingJob returns the most recent processing job for a given document.
	GetLatestProcessingJob(ctx context.Context, docID string) (*domain.DocumentProcessingJob, error)
	// GetProcessingJob retrieves a processing job by its unique identifier.
	GetProcessingJob(ctx context.Context, jobID string) (*domain.DocumentProcessingJob, error)
	// GetJobCapability returns the capability envelope granting permissions and quotas for a job.
	GetJobCapability(ctx context.Context, jobID string) (*domain.JobCapability, error)
	// GetJobExecutionPlan retrieves the persisted execution plan for a processing job.
	GetJobExecutionPlan(ctx context.Context, jobID string) (*domain.JobExecutionPlan, error)
	// UpsertJobExecutionPlan stores or updates the execution plan for a processing job.
	UpsertJobExecutionPlan(ctx context.Context, jobID string, plan *domain.JobExecutionPlan) error
	// UpsertJobEvaluation persists the evaluation report for a completed job.
	UpsertJobEvaluation(ctx context.Context, jobID string, result *domain.JobEvaluationResult) error
	// EvaluateJob computes or retrieves evaluation metrics for a finished processing job.
	EvaluateJob(ctx context.Context, jobID string) (*domain.JobEvaluationResult, error)
	// ListJobApprovalRequests returns all approval requests generated during job execution.
	ListJobApprovalRequests(ctx context.Context, jobID string) ([]*domain.JobApprovalRequest, error)
	// RequestJobApproval registers an approval gate requiring human review before dangerous operations proceed.
	RequestJobApproval(ctx context.Context, jobID, requestedBy, reason string) (*domain.JobApprovalRequest, error)
	// ApproveJobApproval marks an approval request approved by an authorized reviewer.
	ApproveJobApproval(ctx context.Context, jobID, approvalID, reviewedBy string) error
	// RejectJobApproval rejects an approval request with an explanatory reason.
	RejectJobApproval(ctx context.Context, jobID, approvalID, reviewedBy, reason string) error
	// SearchRelatedChunksByVector performs nearest-neighbor vector similarity search over document chunks.
	SearchRelatedChunksByVector(ctx context.Context, workspaceID string, embedding []float32, limit int) ([]*domain.DocumentChunk, error)
	// LogToolCall records execution details and duration of an invoked agent tool.
	LogToolCall(ctx context.Context, jobID, toolName, inputJSON, outputJSON string, durationMs int64) error
	// CreateProcessingJob creates a processing job alongside its default job capability envelope.
	// Because this method does not open a transaction internally, caller must wrap with Transactor.WithTx
	// if atomicity with other operations is required.
	CreateProcessingJob(ctx context.Context, docID, workspaceID, requestedBy string, jobType appv1.JobType) (*domain.DocumentProcessingJob, error)
	// MarkProcessingJobRunning sets a job status to running and updates its start timestamp.
	MarkProcessingJobRunning(ctx context.Context, jobID string) error
	// UpdateProcessingJobStage updates the current execution stage name of a processing job.
	UpdateProcessingJobStage(ctx context.Context, jobID, stage string) error
	// FailProcessingJob transitions a job to failed status with an error message.
	FailProcessingJob(ctx context.Context, jobID, errorMessage string) error
	// RequeueProcessingJobForRetry resets a failed or interrupted job to pending status for retry.
	RequeueProcessingJobForRetry(ctx context.Context, jobID string) error
	// CompleteProcessingJob marks a processing job as successfully completed.
	CompleteProcessingJob(ctx context.Context, jobID string) error
	// SaveDocumentChunks stores parsed text chunks and their embeddings for a document.
	SaveDocumentChunks(ctx context.Context, documentID string, chunks []*domain.DocumentChunk) error
	// ListJobMutationLogs returns all knowledge tree mutations performed by a processing job.
	ListJobMutationLogs(ctx context.Context, jobID string) ([]*domain.JobMutationLog, error)
	// ListAllJobs returns all document processing jobs across all documents.
	ListAllJobs(ctx context.Context) ([]*domain.DocumentProcessingJob, error)
	// ListJobLogs returns paginated structured execution logs for a processing job.
	ListJobLogs(ctx context.Context, jobID string, pageToken string, limit int) ([]*domain.JobLog, string, error)
	// SearchJobLogs searches logs matching specific filter criteria across jobs.
	SearchJobLogs(ctx context.Context, filter domain.JobLogSearchFilter) ([]*domain.JobLog, string, error)
	// ListRelatedJobLogs aggregates job logs across related scopes (workspace, document, or job).
	ListRelatedJobLogs(ctx context.Context, scope domain.RelatedLogScope, workspaceID, documentID, jobID string, pageToken string, limit int) ([]*domain.JobLogGroup, string, error)
}

// TreeRepository handles hierarchical graph retrieval and path exploration across knowledge items.
type TreeRepository interface {
	// GetTreeByWorkspace retrieves all knowledge tree items within a workspace.
	GetTreeByWorkspace(ctx context.Context, wsID string) ([]*domain.Item, error)
	// FindPaths calculates directional traversal paths between two knowledge tree nodes up to maxDepth.
	FindPaths(ctx context.Context, wsID, sourceItemID, targetItemID string, maxDepth, limit int) ([]*domain.Item, []domain.TreePath, error)
	// GetSubtree returns the subtree rooted at rootItemID up to maxDepth levels.
	GetSubtree(ctx context.Context, rootItemID string, maxDepth int) ([]*domain.SubtreeItem, error)
}

// ItemRepository manages knowledge tree nodes, capability-enforced mutations, and item aliases.
type ItemRepository interface {
	// GetItem retrieves a single knowledge item by its unique identifier.
	GetItem(ctx context.Context, itemID string) (*domain.Item, error)
	// CreateItem persists a new knowledge tree node under an optional parent.
	CreateItem(ctx context.Context, workspaceID, label, description, parentID, createdBy string) (*domain.Item, error)
	// CreateStructuredItemWithCapability validates job capabilities, creates a structured knowledge item,
	// and records mutation audit logs. Violations (unpermitted operation, workspace mismatch, quota exceeded,
	// or expired capability) return domain.ErrForbidden. Wrap with Transactor.WithTx if atomic execution is needed.
	CreateStructuredItemWithCapability(ctx context.Context, capability *domain.JobCapability, jobID, documentID, workspaceID, label string, level int, description, summaryHTML, overrideCSS, createdBy, parentID string, sourceChunkIDs []string) (*domain.Item, error)
	// UpsertItemSource records provenance linking a knowledge item to a specific document chunk source text.
	UpsertItemSource(ctx context.Context, itemID, documentID, fileID, chunkID, sourceText string, confidence float64) error
	// UpdateItemSummaryHTMLWithCapability updates an item's rendered HTML summary under capability verification.
	UpdateItemSummaryHTMLWithCapability(ctx context.Context, capability *domain.JobCapability, jobID, itemID, summaryHTML string) error
	// MergeItemWithCapability folds a document's knowledge into an existing node (rewriting title, description,
	// content, and marking cross_document). Pair with UpsertItemSource to record provenance.
	MergeItemWithCapability(ctx context.Context, capability *domain.JobCapability, jobID, itemID, title, description, content string) (*domain.Item, error)
	// ApproveAlias promotes an alias relationship to active status.
	ApproveAlias(ctx context.Context, wsID, canonicalItemID, aliasItemID string) error
	// RejectAlias rejects an alias suggestion between items.
	RejectAlias(ctx context.Context, wsID, canonicalItemID, aliasItemID string) error
}

// UsageRepository manages model pricing schedules, usage accounting, daily aggregation, and billing credits.
type UsageRepository interface {
	// GetModelPricing returns pricing rates per input/output token for an LLM model.
	GetModelPricing(ctx context.Context, model string) (*domain.ModelPricing, error)
	// RecordUsageAccounting idempotently records a metered token consumption event and checks budget caps.
	RecordUsageAccounting(ctx context.Context, ev *domain.UsageEvent, date string) (string, bool, error)
	// ListUsageByModel aggregates token consumption and cost broken down by LLM model for a date range.
	ListUsageByModel(ctx context.Context, accountID, periodStart, periodEnd string) ([]domain.ModelUsage, string, error)
	// ListDailyUsage aggregates daily expenditure and token counts over a billing period.
	ListDailyUsage(ctx context.Context, accountID, periodStart, periodEnd, currency string) ([]domain.DailyUsage, error)
	// UpdateAccountBudgetLimit adjusts the spending limit cap for an account in minor currency units.
	UpdateAccountBudgetLimit(ctx context.Context, accountID string, limitMinor int64) error
	// ListInvoices returns historical invoices generated for an account.
	ListInvoices(ctx context.Context, accountID string, limit int) (*domain.InvoiceList, error)
	// ListPaymentMethods returns saved payment methods attached to a billing account.
	ListPaymentMethods(ctx context.Context, accountID string) ([]*domain.PaymentMethod, error)

	// GrantCredit issues a promotional or prepaid credit grant to an account.
	GrantCredit(ctx context.Context, grant *domain.CreditGrant) error
	// GetCreditBalance returns the remaining usable credit balance in minor currency units.
	GetCreditBalance(ctx context.Context, accountID string) (int64, error)
	// ListCreditGrants lists credit grants awarded to an account up to limit.
	ListCreditGrants(ctx context.Context, accountID string, limit int) ([]*domain.CreditGrant, error)
}

// JobLogWriter provides a sink for structured execution events emitted by running jobs.
type JobLogWriter interface {
	// LogJobEvent persists a structured job execution event to log storage.
	LogJobEvent(ctx context.Context, e joblog.Event) error
}

// CheckpointRepository persists pipeline execution stage progression for resume-on-failure capabilities.
type CheckpointRepository interface {
	// UpsertStageRunning marks a pipeline stage as currently running.
	UpsertStageRunning(ctx context.Context, jobID, stage string) error
	// MarkStageSucceeded records successful completion of a stage with an optional GCS artifact reference.
	MarkStageSucceeded(ctx context.Context, jobID, stage, gcsRef string) error
	// MarkStageFailed records stage execution failure along with the error reason.
	MarkStageFailed(ctx context.Context, jobID, stage, errorMessage string) error
	// ListStageCheckpoints returns all persisted stage checkpoint records for a job.
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
