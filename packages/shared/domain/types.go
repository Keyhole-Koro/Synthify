package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	treev1 "github.com/synthify/backend/packages/shared/gen/synthify/tree/v1"
	"github.com/synthify/backend/packages/shared/util"
)

var (
	ErrNotFound             = errors.New("not found")
	ErrApprovalRequired     = errors.New("job execution plan requires approval")
	ErrPlanRejected         = errors.New("job execution plan was rejected")
	ErrNotImplemented       = errors.New("not implemented")
	ErrFileTooLarge         = errors.New("file exceeds max file size")
	ErrStorageQuotaExceeded = errors.New("storage quota exceeded")
	ErrUploadNotConfirmed   = errors.New("upload is not confirmed")
	ErrUploadSizeMismatch   = errors.New("uploaded object size does not match reservation")

	// Severity markers
	ErrCritical = errors.New("critical system error") // Triggers CRITICAL alert
	ErrJobError = errors.New("job level error")       // Triggers ERROR notification

	// Checkpoint exceptions
	ErrCheckpointNotFound = errors.New("checkpoint not found")
	ErrCheckpointInvalid  = errors.New("checkpoint version or format is invalid")
	ErrCheckpointMismatch = errors.New("checkpoint inputs do not match current request")
)

type ExecutePlanRequest struct {
	JobID       string `json:"job_id"`
	JobType     string `json:"job_type"`
	DocumentID  string `json:"document_id"`
	WorkspaceID string `json:"workspace_id"`
	TreeID      string `json:"tree_id"`
	FileURI     string `json:"file_uri"`
	Filename    string `json:"filename"`
	MimeType    string `json:"mime_type"`
}

func (req ExecutePlanRequest) Validate() error {
	switch {
	case req.JobID == "":
		return fmt.Errorf("job_id is required")
	case req.DocumentID == "":
		return fmt.Errorf("document_id is required")
	case req.WorkspaceID == "":
		return fmt.Errorf("workspace_id is required")
	default:
		return nil
	}
}

type SynthesizedItem struct {
	LocalID        string   `json:"local_id"`
	Title          string   `json:"title"`
	Level          int      `json:"level"`
	Description    string   `json:"description"`
	Content        string   `json:"content"`
	OverrideCSS    string   `json:"override_css,omitempty"`
	ParentLocalID  string   `json:"parent_local_id"`
	ChildLocalIDs  []string `json:"child_local_ids"`
	SourceChunkIDs []string `json:"source_chunk_ids"`
}

type SourceFile struct {
	WorkspaceID string `json:"workspace_id,omitempty"`
	DocumentID  string `json:"document_id,omitempty"`
	Filename    string `json:"filename"`
	URI         string `json:"uri"`
	MimeType    string `json:"mime_type"`
	Content     []byte `json:"content,omitempty"`
}

type Chunk struct {
	ChunkIndex int    `json:"chunk_index"`
	Heading    string `json:"heading"`
	Text       string `json:"text"`
}

type DocumentBrief struct {
	Topic        string   `json:"topic"`
	Level01Hints []string `json:"level01_hints"`
	ClaimSummary string   `json:"claim_summary"`
	Entities     []string `json:"entities"`
	Outline      []string `json:"outline"`
}

type SectionBrief struct {
	Heading         string   `json:"heading"`
	Topic           string   `json:"topic"`
	ItemCandidates  []string `json:"item_candidates"`
	ConnectionHints string   `json:"connection_hints"`
}

type Account struct {
	AccountID            string `json:"account_id"`
	Name                 string `json:"name"`
	Plan                 string `json:"plan"` // "free" | "pro"
	StorageQuotaBytes    int64  `json:"storage_quota_bytes"`
	StorageUsedBytes     int64  `json:"storage_used_bytes"`
	MaxFileSizeBytes     int64  `json:"max_file_size_bytes"`
	MaxUploadsPerFiveH   int64  `json:"max_uploads_per_5h"`
	MaxUploadsPerWeek    int64  `json:"max_uploads_per_1week"`
	StripeCustomerID     string `json:"stripe_customer_id,omitempty"`
	StripeSubscriptionID string `json:"stripe_subscription_id,omitempty"`
	BillingStatus        string `json:"billing_status,omitempty"`
	StripePriceID        string `json:"stripe_price_id,omitempty"`
	BillingCurrency      string `json:"billing_currency,omitempty"`
	BillingAmountMinor   int64  `json:"billing_amount_minor,omitempty"`
	BillingInterval      string `json:"billing_interval,omitempty"`
	CurrentPeriodEnd     string `json:"current_period_end,omitempty"`
	CancelAtPeriodEnd    bool   `json:"cancel_at_period_end,omitempty"`
	BillingUpdatedAt     string `json:"billing_updated_at,omitempty"`
	CreatedAt            string `json:"created_at"`
}

type AccountUser struct {
	AccountID string `json:"account_id"`
	UserID    string `json:"user_id"`
	Role      string `json:"role"`
	JoinedAt  string `json:"joined_at"`
}

type Workspace struct {
	WorkspaceID        string `json:"workspace_id"`
	AccountID          string `json:"account_id"`
	Name               string `json:"name"`
	Plan               string `json:"plan,omitempty"`
	StorageUsedBytes   int64  `json:"storage_used_bytes,omitempty"`
	StorageQuotaBytes  int64  `json:"storage_quota_bytes,omitempty"`
	MaxFileSizeBytes   int64  `json:"max_file_size_bytes,omitempty"`
	MaxUploadsPerFiveH int64  `json:"max_uploads_per_5h,omitempty"`
	MaxUploadsPerWeek  int64  `json:"max_uploads_per_1week,omitempty"`
	RootItemID         string `json:"root_item_id,omitempty"`
	CreatedAt          string `json:"created_at"`
}

type Document struct {
	DocumentID  string `json:"document_id"`
	WorkspaceID string `json:"workspace_id"`
	UploadedBy  string `json:"uploaded_by"`
	Filename    string `json:"filename"`
	MimeType    string `json:"mime_type"`
	FileSize    int64  `json:"file_size"`
	CreatedAt   string `json:"created_at"`
}

type DocumentFile struct {
	FileID     string `json:"file_id"`
	DocumentID string `json:"document_id"`
	Path       string `json:"path"`
	MimeType   string `json:"mime_type"`
	FileSize   int64  `json:"file_size"`
	CreatedAt  string `json:"created_at"`
}

type ObjectMetadata struct {
	Size        int64  `json:"size"`
	ContentType string `json:"content_type,omitempty"`
}

type DocumentProcessingJob struct {
	JobID            string                   `json:"job_id"`
	DocumentID       string                   `json:"document_id"`
	WorkspaceID      string                   `json:"workspace_id,omitempty"`
	JobType          treev1.JobType           `json:"job_type"`
	Status           treev1.JobLifecycleState `json:"status"`
	CurrentStage     string                   `json:"current_stage,omitempty"`
	ErrorMessage     string                   `json:"error_message,omitempty"`
	ParamsJSON       string                   `json:"params_json,omitempty"`
	RequestedBy      string                   `json:"requested_by,omitempty"`
	CapabilityID     string                   `json:"capability_id,omitempty"`
	ExecutionPlanID  string                   `json:"execution_plan_id,omitempty"`
	PlanStatus       string                   `json:"plan_status,omitempty"`
	EvaluationStatus string                   `json:"evaluation_status,omitempty"`
	RetryCount       int                      `json:"retry_count,omitempty"`
	BudgetJSON       string                   `json:"budget_json,omitempty"`
	CreatedAt        string                   `json:"created_at"`
	UpdatedAt        string                   `json:"updated_at"`
}

type Tree struct {
	TreeID      string `json:"tree_id"`
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type Item struct {
	ItemID            string                     `json:"id"`
	WorkspaceID       string                     `json:"workspace_id"`
	Title             string                     `json:"title"`
	Level             int                        `json:"level,omitempty"`
	Description       string                     `json:"description"`
	Content           string                     `json:"content,omitempty"`
	OverrideCSS       string                     `json:"override_css,omitempty"`
	CreatedBy         string                     `json:"created_by,omitempty"`
	GovernanceState   treev1.ItemGovernanceState `json:"governance_state,omitempty"`
	LastMutationJobID string                     `json:"last_mutation_job_id,omitempty"`
	CreatedAt         string                     `json:"created_at"`
	ParentID          string                     `json:"parent_id,omitempty"`
	ChildIDs          []string                   `json:"child_ids,omitempty"`
	Scope             treev1.TreeProjectionScope `json:"scope,omitempty"`
}

type ItemSource struct {
	ItemID     string  `json:"item_id"`
	DocumentID string  `json:"document_id"`
	FileID     string  `json:"file_id"`
	ChunkID    string  `json:"chunk_id,omitempty"`
	SourceText string  `json:"source_text,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
}

type ItemEvidence struct {
	Sources []*ItemSource `json:"sources,omitempty"`
}

// TreeItem is the item representation returned by the API.
type TreeItem struct {
	ID          string                     `json:"id"`
	Scope       treev1.TreeProjectionScope `json:"scope"`
	Title       string                     `json:"title"`
	Description string                     `json:"description"`
	Content     string                     `json:"content,omitempty"`
}

type DocumentChunk struct {
	ChunkID    string    `json:"chunk_id"`
	DocumentID string    `json:"document_id"`
	FileID     string    `json:"file_id"`
	Heading    string    `json:"heading"`
	Text       string    `json:"text"`
	SourcePage int       `json:"source_page,omitempty"`
	Embedding  []float32 `json:"embedding,omitempty"`
}

type Job struct {
	JobID  string `json:"job_id"`
	Status string `json:"status"`
}

type JobLog struct {
	Timestamp   string `json:"timestamp"`
	Level       string `json:"level"`
	Event       string `json:"event"`
	Message     string `json:"message"`
	DetailJSON  string `json:"detail_json"`
	Source      string `json:"source"`
	SourceID    string `json:"source_id"`
	JobID       string `json:"job_id"`
	DocumentID  string `json:"document_id"`
	WorkspaceID string `json:"workspace_id"`
}

type JobLogSearchFilter struct {
	Query         string
	WorkspaceID   string
	DocumentID    string
	JobID         string
	Levels        []string
	Events        []string
	FromTimestamp string
	ToTimestamp   string
	PageToken     string
	Limit         int
}

type RelatedLogScope string

const (
	RelatedLogScopeJob       RelatedLogScope = "job"
	RelatedLogScopeDocument  RelatedLogScope = "document"
	RelatedLogScopeWorkspace RelatedLogScope = "workspace"
)

type JobLogJob struct {
	JobID     string                   `json:"job_id"`
	Status    treev1.JobLifecycleState `json:"status"`
	CreatedAt string                   `json:"created_at"`
	Logs      []*JobLog                `json:"logs"`
}

type JobLogGroup struct {
	WorkspaceID string       `json:"workspace_id"`
	DocumentID  string       `json:"document_id"`
	Jobs        []*JobLogJob `json:"jobs"`
}

type NodeGovernanceState string

type JobCapability struct {
	CapabilityID       string                `json:"capability_id"`
	JobID              string                `json:"job_id"`
	WorkspaceID        string                `json:"workspace_id"`
	AllowedDocumentIDs []string              `json:"allowed_document_ids,omitempty"`
	AllowedItemIDs     []string              `json:"allowed_item_ids,omitempty"`
	AllowedOperations  []treev1.JobOperation `json:"allowed_operations,omitempty"`
	MaxLLMCalls        int                   `json:"max_llm_calls,omitempty"`
	MaxToolRuns        int                   `json:"max_tool_runs,omitempty"`
	MaxItemCreations   int                   `json:"max_item_creations,omitempty"`
	ExpiresAt          string                `json:"expires_at,omitempty"`
	CreatedAt          string                `json:"created_at,omitempty"`
}

func (c *JobCapability) Allows(op treev1.JobOperation) bool {
	if c == nil {
		return false
	}
	for _, allowed := range c.AllowedOperations {
		if allowed == op {
			return true
		}
	}
	return false
}

func (c *JobCapability) AllowsDocument(documentID string) bool {
	if c == nil || documentID == "" {
		return false
	}
	if len(c.AllowedDocumentIDs) == 0 {
		return true
	}
	for _, allowed := range c.AllowedDocumentIDs {
		if allowed == documentID {
			return true
		}
	}
	return false
}

func (c *JobCapability) AllowsItem(itemID string) bool {
	if c == nil || itemID == "" {
		return false
	}
	if len(c.AllowedItemIDs) == 0 {
		return true
	}
	for _, allowed := range c.AllowedItemIDs {
		if allowed == itemID {
			return true
		}
	}
	return false
}

func (c *JobCapability) IsExpired(now time.Time) bool {
	if c == nil || c.ExpiresAt == "" {
		return false
	}
	expiresAt, err := time.Parse(time.RFC3339, c.ExpiresAt)
	if err != nil {
		return true
	}
	return now.After(expiresAt)
}

type JobMutationLog struct {
	MutationID     string `json:"mutation_id"`
	JobID          string `json:"job_id"`
	PlanID         string `json:"plan_id,omitempty"`
	CapabilityID   string `json:"capability_id,omitempty"`
	WorkspaceID    string `json:"workspace_id"`
	TargetType     string `json:"target_type"`
	TargetID       string `json:"target_id"`
	MutationType   string `json:"mutation_type"`
	RiskTier       string `json:"risk_tier,omitempty"`
	BeforeJSON     string `json:"before_json,omitempty"`
	AfterJSON      string `json:"after_json,omitempty"`
	ProvenanceJSON string `json:"provenance_json,omitempty"`
	CreatedAt      string `json:"created_at"`
}

type JobExecutionPlan struct {
	PlanID    string `json:"plan_id"`
	JobID     string `json:"job_id"`
	Status    string `json:"status"`
	Summary   string `json:"summary,omitempty"`
	PlanJSON  string `json:"plan_json,omitempty"`
	CreatedBy string `json:"created_by,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type JobPlanningSignals struct {
	DocumentID            string `json:"document_id,omitempty"`
	WorkspaceID           string `json:"workspace_id,omitempty"`
	SameDocumentItemCount int    `json:"same_document_item_count,omitempty"`
	ApprovedAliasCount    int    `json:"approved_alias_count,omitempty"`
	ProtectedAliasCount   int    `json:"protected_alias_count,omitempty"`
}

type JobEvaluationResult struct {
	JobID         string   `json:"job_id"`
	PlanID        string   `json:"plan_id,omitempty"`
	Passed        bool     `json:"passed"`
	Status        string   `json:"status,omitempty"`
	Summary       string   `json:"summary,omitempty"`
	Score         int32    `json:"score,omitempty"`
	Findings      []string `json:"findings,omitempty"`
	MutationCount int32    `json:"mutation_count,omitempty"`
}

type JobApprovalRequest struct {
	ApprovalID          string                `json:"approval_id"`
	JobID               string                `json:"job_id"`
	PlanID              string                `json:"plan_id"`
	Status              string                `json:"status"`
	RequestedOperations []treev1.JobOperation `json:"requested_operations,omitempty"`
	Reason              string                `json:"reason,omitempty"`
	RiskTier            string                `json:"risk_tier,omitempty"`
	RequestedBy         string                `json:"requested_by,omitempty"`
	ReviewedBy          string                `json:"reviewed_by,omitempty"`
	RequestedAt         string                `json:"requested_at,omitempty"`
	ReviewedAt          string                `json:"reviewed_at,omitempty"`
}

type jobExecutionPlanPayload struct {
	Steps []struct {
		RiskTier string `json:"risk_tier"`
	} `json:"steps"`
}

func (p *JobExecutionPlan) HighestRiskTier() string {
	if p == nil || strings.TrimSpace(p.PlanJSON) == "" {
		return "tier_1"
	}
	var payload jobExecutionPlanPayload
	if err := json.Unmarshal([]byte(p.PlanJSON), &payload); err != nil {
		return "tier_1"
	}
	maxRisk := "tier_1"
	for _, step := range payload.Steps {
		risk := util.NormalizeRiskTier(step.RiskTier)
		if riskTierRank(risk) > riskTierRank(maxRisk) {
			maxRisk = risk
		}
	}
	return maxRisk
}

func (p *JobExecutionPlan) RequiresApproval() bool {
	return riskTierRank(p.HighestRiskTier()) >= riskTierRank("tier_2")
}

func riskTierRank(value string) int {
	switch util.NormalizeRiskTier(value) {
	case "tier_3":
		return 3
	case "tier_2":
		return 2
	default:
		return 1
	}
}

func DefaultJobCapability(jobID, workspaceID, documentID string, createdAt time.Time) *JobCapability {
	return &JobCapability{
		CapabilityID:       "cap_" + jobID,
		JobID:              jobID,
		WorkspaceID:        workspaceID,
		AllowedDocumentIDs: []string{documentID},
		AllowedOperations: []treev1.JobOperation{
			treev1.JobOperation_JOB_OPERATION_READ_TREE,
			treev1.JobOperation_JOB_OPERATION_READ_DOCUMENT,
			treev1.JobOperation_JOB_OPERATION_CREATE_ITEM,
			treev1.JobOperation_JOB_OPERATION_UPDATE_ITEM,
			treev1.JobOperation_JOB_OPERATION_INVOKE_LLM,
		},
		MaxLLMCalls:      128,
		MaxToolRuns:      0,
		MaxItemCreations: 4096,
		ExpiresAt:        createdAt.Add(24 * time.Hour).UTC().Format(time.RFC3339),
		CreatedAt:        createdAt.UTC().Format(time.RFC3339),
	}
}

type SubtreeItem struct {
	Item
	HasChildren bool `json:"has_children"`
}

type TreePath struct {
	ItemIDs  []string `json:"item_ids"`
	HopCount int      `json:"hop_count"`
	Evidence struct {
		SourceDocumentIDs []string `json:"source_document_ids"`
	} `json:"evidence_ref"`
}

type JobStageCheckpoint struct {
	JobID     string `json:"job_id"`
	Stage     string `json:"stage"`
	Status    string `json:"status"` // "running" | "succeeded" | "failed"
	GCSRef    string `json:"gcs_ref"`
	UpdatedAt string `json:"updated_at"`
}

type CheckpointEnvelope struct {
	SchemaVersion int            `json:"schema_version"`
	Kind          string         `json:"kind"` // "synthify.worker_checkpoint"
	Stage         string         `json:"stage"`
	JobID         string         `json:"job_id"`
	DocumentID    string         `json:"document_id"`
	WorkspaceID   string         `json:"workspace_id"`
	CreatedAt     string         `json:"created_at"`
	Inputs        map[string]any `json:"inputs"`
	Outputs       map[string]any `json:"outputs"`
	Stats         map[string]any `json:"stats"`
}
