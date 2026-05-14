package repository

import (
	"context"

	"github.com/synthify/backend/packages/shared/domain"
	treev1 "github.com/synthify/backend/packages/shared/gen/synthify/tree/v1"
)

type DocumentUploadURLBuilder func(workspaceID, objectName string) string
type DocumentSourceURLBuilder func(workspaceID, documentID string) string

type AccountRepository interface {
	GetOrCreateAccount(ctx context.Context, userID string) (*domain.Account, error)
	GetAccount(ctx context.Context, accountID string) (*domain.Account, error)
	IsAccountAccessible(ctx context.Context, accountID, userID string) bool
	SetAccountStripeCustomerID(ctx context.Context, accountID, stripeCustomerID string) error
	ApplyBillingPlan(ctx context.Context, accountID, stripeCustomerID, stripeSubscriptionID string, plan domain.BillingPlan) error
	ApplyBillingPlanByStripeCustomerID(ctx context.Context, stripeCustomerID, stripeSubscriptionID string, plan domain.BillingPlan) error
	RecordBillingWebhookEvent(ctx context.Context, event *domain.ProviderWebhookEvent) (bool, error)
	MarkBillingWebhookEventProcessed(ctx context.Context, provider, eventID, status, errorMessage string) error
	ApplyBillingEvent(ctx context.Context, event *domain.ProviderWebhookEvent) error
}

type WorkspaceRepository interface {
	ListWorkspacesByUser(ctx context.Context, userID string) []*domain.Workspace
	GetWorkspace(ctx context.Context, id string) (*domain.Workspace, error)
	IsWorkspaceAccessible(ctx context.Context, wsID, userID string) bool
	CreateWorkspace(ctx context.Context, accountID, name string) *domain.Workspace
}

type DocumentRepository interface {
	ListDocuments(ctx context.Context, wsID string) []*domain.Document
	GetDocument(ctx context.Context, id string) (*domain.Document, error)
	GetDocumentChunks(ctx context.Context, documentID string) ([]*domain.DocumentChunk, error)
	GetJobPlanningSignals(ctx context.Context, documentID, workspaceID, treeID string) (*domain.JobPlanningSignals, error)
	CreateDocument(ctx context.Context, wsID, uploadedBy, filename, mimeType string, fileSize int64) (*domain.Document, string, error)
	ConfirmDocumentUpload(ctx context.Context, documentID string, actualSize int64) error
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
	CreateProcessingJob(ctx context.Context, docID, workspaceID string, jobType treev1.JobType) *domain.DocumentProcessingJob
	MarkProcessingJobRunning(ctx context.Context, jobID string) error
	UpdateProcessingJobStage(ctx context.Context, jobID, stage string) error
	FailProcessingJob(ctx context.Context, jobID, errorMessage string) error
	CompleteProcessingJob(ctx context.Context, jobID string) error
	SaveDocumentChunks(ctx context.Context, documentID string, chunks []*domain.DocumentChunk) error
	ListJobMutationLogs(ctx context.Context, jobID string) ([]*domain.JobMutationLog, error)
	ListAllJobs(ctx context.Context) ([]*domain.DocumentProcessingJob, error)
	ListJobLogs(ctx context.Context, jobID string, pageToken string, limit int) ([]*domain.JobLog, string, error)
	SearchJobLogs(ctx context.Context, filter domain.JobLogSearchFilter) ([]*domain.JobLog, string, error)
	ListRelatedJobLogs(ctx context.Context, scope domain.RelatedLogScope, workspaceID, documentID, jobID string, pageToken string, limit int) ([]*domain.JobLogGroup, string, error)
}

type TreeRepository interface {
	GetOrCreateTree(ctx context.Context, wsID string) (*domain.Tree, error)
	GetTreeByWorkspace(ctx context.Context, wsID string) ([]*domain.Item, error)
	GetWorkspaceRootItemID(ctx context.Context, wsID string) (string, error)
	FindPaths(ctx context.Context, wsID, sourceItemID, targetItemID string, maxDepth, limit int) ([]*domain.Item, []domain.TreePath, error)
	GetSubtree(ctx context.Context, rootItemID string, maxDepth int) ([]*domain.SubtreeItem, error)
}

type ItemRepository interface {
	GetItem(ctx context.Context, itemID string) (*domain.Item, error)
	CreateItem(ctx context.Context, workspaceID, label, description, parentID, createdBy string) *domain.Item
	CreateStructuredItemWithCapability(ctx context.Context, capability *domain.JobCapability, jobID, documentID, workspaceID, label string, level int, description, summaryHTML, overrideCSS, createdBy, parentID string, sourceChunkIDs []string) *domain.Item
	UpsertItemSource(ctx context.Context, itemID, documentID, fileID, chunkID, sourceText string, confidence float64) error
	UpdateItemSummaryHTMLWithCapability(ctx context.Context, capability *domain.JobCapability, jobID, itemID, summaryHTML string) error
	ApproveAlias(ctx context.Context, wsID, canonicalItemID, aliasItemID string) error
	RejectAlias(ctx context.Context, wsID, canonicalItemID, aliasItemID string) error
}

type CheckpointRepository interface {
	UpsertStageRunning(ctx context.Context, jobID, stage string) error
	MarkStageSucceeded(ctx context.Context, jobID, stage, gcsRef string) error
	MarkStageFailed(ctx context.Context, jobID, stage, errorMessage string) error
	ListStageCheckpoints(ctx context.Context, jobID string) ([]domain.JobStageCheckpoint, error)
}
