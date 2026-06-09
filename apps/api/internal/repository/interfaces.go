package repository

import (
	"context"
	"time"

	"github.com/synthify/backend/apps/api/internal/domain"
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
	// IssueDocumentUploadURL issues a signed PUT URL to GCS. fileSize is bound
	// into the signature via the Content-Length header; it is always validated
	// to be positive by the caller (CreateDocument) before reaching here.
	IssueDocumentUploadURL(ctx context.Context, workspaceID, objectName, contentType string, fileSize int64) (DocumentUploadTarget, error)
}

// SignedURL is a short-lived signed GET URL plus its expiry.
type SignedURL struct {
	URL       string
	ExpiresAt time.Time
}

// DocumentImageURLIssuer issues a short-lived signed GET URL for a document's
// uploaded original, used to render images embedded in generated HTML by their
// data-file-id marker. The url is minted fresh per view and expires, so it is
// never persisted into the saved HTML.
type DocumentImageURLIssuer interface {
	IssueDocumentImageURL(ctx context.Context, workspaceID, documentID, filename string) (SignedURL, error)
}

// DocumentObjectStore は CreateDocument で発行された GCS object に対する
// 後始末を担う。reservation の mismatch / expiry のような「アップロード失敗」
// 系で残骸を消すために用意している。
type DocumentObjectStore interface {
	DeleteDocumentObject(ctx context.Context, workspaceID, objectName string) error
}

// Repositories は単一 unit-of-work 内で利用可能な全リポジトリの集合体。
// 個別の repo interface を埋め込むことで、tx スコープ内のクロスリポジトリ
// 操作 (例: JobCapability + ProcessingJob を同一 tx で挿入) を表現する。
// postgres / mock Store のどちらもこの interface を満たす。
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
}

// Transactor は service 層から tx 境界を制御するためのインターフェース。
// fn が nil を返せば commit、エラーを返せば rollback される。
// 渡される Repositories は tx スコープに束縛されている。
type Transactor interface {
	WithTx(ctx context.Context, fn func(Repositories) error) error
}

type DocumentSourceURLBuilder func(workspaceID, documentID string) string

type AccountRepository interface {
	GetOrCreateAccount(ctx context.Context, userID string) (*domain.Account, error)
	GetAccount(ctx context.Context, accountID string) (*domain.Account, error)
	GetAccountByUser(ctx context.Context, userID string) (*domain.Account, error)
	CreateAccount(ctx context.Context, userID string) (*domain.Account, error)
	// IsAccountAccessible は DB エラーと「アクセス不可」を区別して返す。
	IsAccountAccessible(ctx context.Context, accountID, userID string) (ok bool, err error)
	SetAccountStripeCustomerID(ctx context.Context, accountID, stripeCustomerID string) error
	ListStripeLinkedAccounts(ctx context.Context, limit int) ([]*domain.Account, error)
	ApplyBillingPlan(ctx context.Context, accountID, stripeCustomerID, stripeSubscriptionID string, plan domain.BillingPlan) error
	ApplyBillingPlanByStripeCustomerID(ctx context.Context, stripeCustomerID, stripeSubscriptionID string, plan domain.BillingPlan) error
	RecordBillingWebhookEvent(ctx context.Context, event *domain.ProviderWebhookEvent) (bool, error)
	MarkBillingWebhookEventProcessed(ctx context.Context, provider, eventID, status, errorMessage string) error
	ApplyBillingEvent(ctx context.Context, event *domain.ProviderWebhookEvent) error
}

type UserRepository interface {
	GetUser(ctx context.Context, userID string) (*domain.User, error)
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
	UpsertUser(ctx context.Context, user *domain.User) (*domain.User, error)
}

type WorkspaceRepository interface {
	ListWorkspacesByUser(ctx context.Context, userID string) ([]*domain.Workspace, error)
	GetWorkspace(ctx context.Context, id string) (*domain.Workspace, error)
	// IsWorkspaceAccessible は DB エラーと「アクセス不可」を区別して返す。
	// err != nil の場合 ok の値は意味を持たない。
	IsWorkspaceAccessible(ctx context.Context, wsID, userID string) (ok bool, err error)
	// GetWorkspaceRole は user の workspace 内 role を返す。account 経由 (所有者) は
	// owner 相当、workspace_members に居ればその role。どちらでもなければ空文字。
	// err != nil の場合 role の値は意味を持たない。
	GetWorkspaceRole(ctx context.Context, wsID, userID string) (domain.WorkspaceRole, error)
	// CreateWorkspace は workspaces + tree root item を 1 ペアで作成する。
	// 内部で tx を張らないため、atomic 性が必要なら呼び出し側を
	// Transactor.WithTx で包むこと。
	CreateWorkspace(ctx context.Context, accountID, name string) (*domain.Workspace, error)
	UpdateWorkspaceName(ctx context.Context, workspaceID, name string) (*domain.Workspace, error)
	DeleteWorkspace(ctx context.Context, workspaceID string) error
	// UpsertWorkspaceMember は招待メンバーを追加 / role 更新する。
	UpsertWorkspaceMember(ctx context.Context, wsID, userID string, role domain.WorkspaceRole, invitedBy string) error
	// ListWorkspaceMembers は workspace の招待メンバー一覧を返す (email 付き)。
	ListWorkspaceMembers(ctx context.Context, wsID string) ([]*domain.WorkspaceMember, error)
	// RemoveWorkspaceMember はメンバーを削除する。対象が居なければ ErrNotFound。
	RemoveWorkspaceMember(ctx context.Context, wsID, userID string) error
	// CreateShareLink は公開リンクを作成する。token は呼び出し側で生成して渡す。
	CreateShareLink(ctx context.Context, link *domain.ShareLink) error
	// ListShareLinks は workspace の全リンク (失効済み含む) を新しい順で返す。
	ListShareLinks(ctx context.Context, wsID string) ([]*domain.ShareLink, error)
	// RevokeShareLink はリンクを失効させる。対象が無い / 既に失効済みなら ErrNotFound。
	RevokeShareLink(ctx context.Context, wsID, token string, now time.Time) error
	// ResolveShareLink は有効な (未失効・未期限切れ) リンクを返す。無効なら ErrNotFound。
	ResolveShareLink(ctx context.Context, token string, now time.Time) (*domain.ShareLink, error)
}

// DocumentRepository は document/file 自体のメタデータ操作を扱う。
// Job / Chunk / Log は別 interface に分離されている (docs/improvements/api-document-repository-split.md)。
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

// DocumentChunkRepository はドキュメントを分割した chunk と vector 検索を扱う。
type DocumentChunkRepository interface {
	GetDocumentChunks(ctx context.Context, documentID string) ([]*domain.DocumentChunk, error)
	SaveDocumentChunks(ctx context.Context, documentID string, chunks []*domain.DocumentChunk) error
	SearchRelatedChunksByVector(ctx context.Context, workspaceID string, embedding []float32, limit int) ([]*domain.DocumentChunk, error)
}

// JobRepository は document processing job の lifecycle と planning を扱う。
// (approval / log / tool-call は別 interface)
type JobRepository interface {
	GetLatestProcessingJob(ctx context.Context, docID string) (*domain.DocumentProcessingJob, error)
	GetProcessingJob(ctx context.Context, jobID string) (*domain.DocumentProcessingJob, error)
	GetJobCapability(ctx context.Context, jobID string) (*domain.JobCapability, error)
	GetJobExecutionPlan(ctx context.Context, jobID string) (*domain.JobExecutionPlan, error)
	UpsertJobExecutionPlan(ctx context.Context, jobID string, plan *domain.JobExecutionPlan) error
	UpsertJobEvaluation(ctx context.Context, jobID string, result *domain.JobEvaluationResult) error
	EvaluateJob(ctx context.Context, jobID string) (*domain.JobEvaluationResult, error)
	// CreateProcessingJob は job_capabilities と processing_jobs を 1 ペアで作成する。
	// 内部で tx を張らないため、複数 sqlc 呼び出しの atomic 性が必要な場合は
	// 呼び出し側を Transactor.WithTx で包むこと (service 層の責務)。
	CreateProcessingJob(ctx context.Context, docID, workspaceID, requestedBy string, jobType appv1.JobType) (*domain.DocumentProcessingJob, error)
	MarkProcessingJobRunning(ctx context.Context, jobID string) error
	UpdateProcessingJobStage(ctx context.Context, jobID, stage string) error
	FailProcessingJob(ctx context.Context, jobID, errorMessage string) error
	SetProcessingJobRetryCount(ctx context.Context, jobID string, retryCount int) error
	CompleteProcessingJob(ctx context.Context, jobID string) error
	ListAllJobs(ctx context.Context) ([]*domain.DocumentProcessingJob, error)
	GetJobPlanningSignals(ctx context.Context, documentID, workspaceID, treeID string) (*domain.JobPlanningSignals, error)
}

// JobApprovalRepository は job の人手承認フローを扱う。
type JobApprovalRepository interface {
	ListJobApprovalRequests(ctx context.Context, jobID string) ([]*domain.JobApprovalRequest, error)
	RequestJobApproval(ctx context.Context, jobID, requestedBy, reason string) (*domain.JobApprovalRequest, error)
	ApproveJobApproval(ctx context.Context, jobID, approvalID, reviewedBy string) error
	RejectJobApproval(ctx context.Context, jobID, approvalID, reviewedBy, reason string) error
}

// JobLogRepository は job 実行ログ / mutation ログ / tool 呼び出し記録を扱う。
type JobLogRepository interface {
	LogToolCall(ctx context.Context, jobID, toolName, inputJSON, outputJSON string, durationMs int64) error
	ListJobMutationLogs(ctx context.Context, jobID string) ([]*domain.JobMutationLog, error)
	ListJobLogs(ctx context.Context, jobID string, pageToken string, limit int) ([]*domain.JobLog, string, error)
	SearchJobLogs(ctx context.Context, filter domain.JobLogSearchFilter) ([]*domain.JobLog, string, error)
	ListRelatedJobLogs(ctx context.Context, scope domain.RelatedLogScope, workspaceID, documentID, jobID string, pageToken string, limit int) ([]*domain.JobLogGroup, string, error)
}

type TreeRepository interface {
	GetTree(ctx context.Context, wsID string) (*domain.Tree, error)
	GetTreeByWorkspace(ctx context.Context, wsID string) ([]*domain.Item, error)
	FindPaths(ctx context.Context, wsID, sourceItemID, targetItemID string, maxDepth, limit int) ([]*domain.Item, []domain.TreePath, error)
	GetSubtree(ctx context.Context, rootItemID string, maxDepth int) ([]*domain.SubtreeItem, error)
}

type ItemRepository interface {
	GetItem(ctx context.Context, itemID string) (*domain.Item, error)
	CreateItem(ctx context.Context, workspaceID, label, description, parentID, createdBy string) (*domain.Item, error)
	// CreateStructuredItemWithCapability は capability の検証 + item 挿入 +
	// mutation log 記録を行う。capability 違反 (op 不許可、ws 不一致、回数上限超過、
	// expired) は domain.ErrForbidden を返す。atomic 性が必要なら呼び出し側を
	// Transactor.WithTx で包むこと。
	CreateStructuredItemWithCapability(ctx context.Context, capability *domain.JobCapability, jobID, documentID, workspaceID, label string, level int, description, summaryHTML, overrideCSS, createdBy, parentID string, sourceChunkIDs []string) (*domain.Item, error)
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
