package app

import (
	"context"
	"time"

	"github.com/synthify/backend/packages/shared/applog"
	"github.com/synthify/backend/packages/shared/config"
	"github.com/synthify/backend/packages/shared/domain"
	treev1 "github.com/synthify/backend/packages/shared/gen/synthify/tree/v1"
	"github.com/synthify/backend/packages/shared/job/log"
	jobstatus "github.com/synthify/backend/packages/shared/job/status"
	"github.com/synthify/backend/packages/shared/repository"
	"github.com/synthify/backend/packages/shared/repository/mock"
	"github.com/synthify/backend/packages/shared/repository/postgres"
	"github.com/synthify/backend/packages/shared/storage"
)

type AppContext struct {
	Store    Store
	Notifier jobstatus.Notifier
}

func Bootstrap(ctx context.Context, gcsURLBase, firebaseProjectID string, logger applog.Logger) *AppContext {
	if logger == nil {
		logger = applog.NoopLogger{}
	}
	store := InitStore(ctx, NewDocumentUploadURLBuilder(gcsURLBase), logger)
	notifier := jobstatus.NewNotifier(ctx, firebaseProjectID, logger)
	return &AppContext{
		Store:    store,
		Notifier: notifier,
	}
}

type Store interface {
	GetOrCreateAccount(ctx context.Context, userID string) (*domain.Account, error)
	GetAccount(ctx context.Context, accountID string) (*domain.Account, error)
	IsAccountAccessible(ctx context.Context, accountID, userID string) bool
	SetAccountStripeCustomerID(ctx context.Context, accountID, stripeCustomerID string) error
	ApplyBillingPlan(ctx context.Context, accountID, stripeCustomerID, stripeSubscriptionID string, plan domain.BillingPlan) error
	ApplyBillingPlanByStripeCustomerID(ctx context.Context, stripeCustomerID, stripeSubscriptionID string, plan domain.BillingPlan) error
	RecordBillingWebhookEvent(ctx context.Context, event *domain.ProviderWebhookEvent) (bool, error)
	MarkBillingWebhookEventProcessed(ctx context.Context, provider, eventID, status, errorMessage string) error
	ApplyBillingEvent(ctx context.Context, event *domain.ProviderWebhookEvent) error
	ListWorkspacesByUser(ctx context.Context, userID string) []*domain.Workspace
	GetWorkspace(ctx context.Context, id string) (*domain.Workspace, error)
	IsWorkspaceAccessible(ctx context.Context, wsID, userID string) bool
	CreateWorkspace(ctx context.Context, accountID, name string) *domain.Workspace
	ListDocuments(ctx context.Context, wsID string) []*domain.Document
	GetDocument(ctx context.Context, id string) (*domain.Document, error)
	CreateDocumentFile(ctx context.Context, docID, path, mimeType string, fileSize int64) (*domain.DocumentFile, error)
	ListDocumentFiles(ctx context.Context, docID string) ([]*domain.DocumentFile, error)
	GetDocumentFileByPath(ctx context.Context, docID, path string) (*domain.DocumentFile, error)
	GetDocumentChunks(ctx context.Context, documentID string) ([]*domain.DocumentChunk, error)
	GetJobPlanningSignals(ctx context.Context, documentID, workspaceID, treeID string) (*domain.JobPlanningSignals, error)
	CreateDocument(ctx context.Context, wsID, uploadedBy, filename, mimeType string, fileSize int64) (*domain.Document, string, error)
	ConfirmDocumentUpload(ctx context.Context, documentID string, actualSize int64) error
	GetLatestProcessingJob(ctx context.Context, docID string) (*domain.DocumentProcessingJob, error)
	GetProcessingJob(ctx context.Context, jobID string) (*domain.DocumentProcessingJob, error)
	GetJobCapability(ctx context.Context, jobID string) (*domain.JobCapability, error)
	GetJobExecutionPlan(ctx context.Context, jobID string) (*domain.JobExecutionPlan, error)
	UpsertJobExecutionPlan(ctx context.Context, jobID string, plan *domain.JobExecutionPlan) error
	UpsertJobEvaluation(ctx context.Context, jobID string, result *domain.JobEvaluationResult) error
	EvaluateJob(ctx context.Context, jobID string) (*domain.JobEvaluationResult, error)
	ListJobApprovalRequests(ctx context.Context, jobID string) ([]*domain.JobApprovalRequest, error)
	ListJobMutationLogs(ctx context.Context, jobID string) ([]*domain.JobMutationLog, error)
	ListAllJobs(ctx context.Context) ([]*domain.DocumentProcessingJob, error)
	LogJobEvent(ctx context.Context, e joblog.Event) error
	ListJobLogs(ctx context.Context, jobID string, pageToken string, limit int) ([]*domain.JobLog, string, error)
	SearchJobLogs(ctx context.Context, filter domain.JobLogSearchFilter) ([]*domain.JobLog, string, error)
	ListRelatedJobLogs(ctx context.Context, scope domain.RelatedLogScope, workspaceID, documentID, jobID string, pageToken string, limit int) ([]*domain.JobLogGroup, string, error)
	RequestJobApproval(ctx context.Context, jobID, requestedBy, reason string) (*domain.JobApprovalRequest, error)
	ApproveJobApproval(ctx context.Context, jobID, approvalID, reviewedBy string) error
	RejectJobApproval(ctx context.Context, jobID, approvalID, reviewedBy, reason string) error
	CreateProcessingJob(ctx context.Context, docID, workspaceID string, jobType treev1.JobType) *domain.DocumentProcessingJob
	MarkProcessingJobRunning(ctx context.Context, jobID string) error
	UpdateProcessingJobStage(ctx context.Context, jobID, stage string) error
	FailProcessingJob(ctx context.Context, jobID, errorMessage string) error
	CompleteProcessingJob(ctx context.Context, jobID string) error
	UpsertStageRunning(ctx context.Context, jobID, stage string) error
	MarkStageSucceeded(ctx context.Context, jobID, stage, gcsRef string) error
	MarkStageFailed(ctx context.Context, jobID, stage, errorMessage string) error
	ListStageCheckpoints(ctx context.Context, jobID string) ([]domain.JobStageCheckpoint, error)
	SaveDocumentChunks(ctx context.Context, documentID string, chunks []*domain.DocumentChunk) error
	LogToolCall(ctx context.Context, jobID, toolName, inputJSON, outputJSON string, durationMs int64) error
	SearchRelatedChunksByVector(ctx context.Context, workspaceID string, embedding []float32, limit int) ([]*domain.DocumentChunk, error)
	GetOrCreateTree(ctx context.Context, wsID string) (*domain.Tree, error)
	GetTreeByWorkspace(ctx context.Context, wsID string) ([]*domain.Item, error)
	GetWorkspaceRootItemID(ctx context.Context, wsID string) (string, error)
	FindPaths(ctx context.Context, wsID, sourceItemID, targetItemID string, maxDepth, limit int) ([]*domain.Item, []domain.TreePath, error)
	GetSubtree(ctx context.Context, rootItemID string, maxDepth int) ([]*domain.SubtreeItem, error)
	GetItem(ctx context.Context, itemID string) (*domain.Item, error)
	CreateItem(ctx context.Context, workspaceID, label, description, parentID, createdBy string) *domain.Item
	CreateStructuredItemWithCapability(ctx context.Context, capability *domain.JobCapability, jobID, documentID, workspaceID, label string, level int, description, summaryHTML, overrideCSS, createdBy, parentID string, sourceChunkIDs []string) *domain.Item
	UpsertItemSource(ctx context.Context, itemID, documentID, subPath, chunkID, sourceText string, confidence float64) error
	UpdateItemSummaryHTMLWithCapability(ctx context.Context, capability *domain.JobCapability, jobID, itemID, summaryHTML string) error
	ApproveAlias(ctx context.Context, wsID, canonicalItemID, aliasItemID string) error
	RejectAlias(ctx context.Context, wsID, canonicalItemID, aliasItemID string) error
}

func NewDocumentUploadURLBuilder(base string) repository.DocumentUploadURLBuilder {
	return func(workspaceID, documentID string) string {
		return storage.BuildDocumentUploadURL(base, workspaceID, documentID)
	}
}

func NewDocumentSourceURLBuilder(base string) repository.DocumentSourceURLBuilder {
	return func(workspaceID, documentID string) string {
		return storage.BuildDocumentSourceURL(base, workspaceID, documentID)
	}
}

func InitStore(ctx context.Context, uploadURLBuilder repository.DocumentUploadURLBuilder, logger applog.Logger) Store {
	if logger == nil {
		logger = applog.NoopLogger{}
	}
	if dsn := config.LoadStore().DatabaseURL; dsn != "" {
		var lastErr error
		for attempt := 1; attempt <= 10; attempt++ {
			store, err := postgres.NewStore(ctx, dsn, uploadURLBuilder, logger)
			if err == nil {
				logger.Info(ctx, "app.store_initialized", map[string]any{"type": "postgres"})
				return store
			}
			lastErr = err
			logger.Warn(ctx, "app.store_init_retry", err, map[string]any{"attempt": attempt})
			time.Sleep(2 * time.Second)
		}
		logger.Error(ctx, "app.store_init_failed", lastErr, nil)
		panic(lastErr)
	}
	logger.Info(ctx, "app.store_initialized", map[string]any{"type": "mock"})
	return mock.NewStore()
}
