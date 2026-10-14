package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
	"github.com/synthify/backend/internal/platform/util"
)

// Job represents a minimal job status identifier.
type Job struct {
	JobID  string `json:"job_id"`
	Status string `json:"status"`
}

// DocumentProcessingJob represents an asynchronous document processing and taxonomy generation job.
type DocumentProcessingJob struct {
	JobID            string                  `json:"job_id"`
	DocumentID       string                  `json:"document_id"`
	WorkspaceID      string                  `json:"workspace_id,omitempty"`
	JobType          appv1.JobType           `json:"job_type"`
	Status           appv1.JobLifecycleState `json:"status"`
	CurrentStage     string                  `json:"current_stage,omitempty"`
	ErrorMessage     string                  `json:"error_message,omitempty"`
	ParamsJSON       string                  `json:"params_json,omitempty"`
	RequestedBy      string                  `json:"requested_by,omitempty"`
	CapabilityID     string                  `json:"capability_id,omitempty"`
	ExecutionPlanID  string                  `json:"execution_plan_id,omitempty"`
	PlanStatus       string                  `json:"plan_status,omitempty"`
	EvaluationStatus string                  `json:"evaluation_status,omitempty"`
	RetryCount       int                     `json:"retry_count,omitempty"`
	BudgetJSON       string                  `json:"budget_json,omitempty"`
	CreatedAt        string                  `json:"created_at"`
	UpdatedAt        string                  `json:"updated_at"`
}

// JobCapability defines the scope, allowed operations, and resource quotas assigned to a running job.
type JobCapability struct {
	CapabilityID       string               `json:"capability_id"`
	JobID              string               `json:"job_id"`
	WorkspaceID        string               `json:"workspace_id"`
	AllowedDocumentIDs []string             `json:"allowed_document_ids,omitempty"`
	AllowedItemIDs     []string             `json:"allowed_item_ids,omitempty"`
	AllowedOperations  []appv1.JobOperation `json:"allowed_operations,omitempty"`
	MaxLLMCalls        int                  `json:"max_llm_calls,omitempty"`
	MaxToolRuns        int                  `json:"max_tool_runs,omitempty"`
	MaxItemCreations   int                  `json:"max_item_creations,omitempty"`
	// MaxTransformCreations caps create_transform invocations per job; the
	// same counter is consumed by syntax_error fix-regeneration so there is
	// no infinite retry loop. MaxTransformRuns caps dynamic-tool executions
	// (ephemeral + promoted) separately from MaxToolRuns. See
	// docs/improvements/dynamic-tool-synthesis.md "コストと再帰の暴走防止".
	MaxTransformCreations int    `json:"max_transform_creations,omitempty"`
	MaxTransformRuns      int    `json:"max_transform_runs,omitempty"`
	ExpiresAt             string `json:"expires_at,omitempty"`
	CreatedAt             string `json:"created_at,omitempty"`
}

// Allows returns whether the specified operation is permitted by this capability.
func (c *JobCapability) Allows(op appv1.JobOperation) bool {
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

// AllowsDocument returns whether the capability permits operations on the specified document ID.
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

// AllowsItem returns whether the capability permits operations targeting the specified item ID.
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

// IsExpired checks whether the capability has expired past its validity window.
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

// JobExecutionPlan describes the multi-stage execution plan and associated risk tier for a job.
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

// jobExecutionPlanPayload models the JSON structure of serialized execution plan steps.
type jobExecutionPlanPayload struct {
	Steps []struct {
		RiskTier string `json:"risk_tier"`
	} `json:"steps"`
}

// HighestRiskTier returns the highest risk tier among all plan steps.
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

// RequiresApproval determines if the plan risk tier requires human approval before proceeding.
func (p *JobExecutionPlan) RequiresApproval() bool {
	return riskTierRank(p.HighestRiskTier()) >= riskTierRank("tier_2")
}

// riskTierRank assigns a numerical ordering to risk tiers for comparison.
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

// JobMutationLog audits individual graph changes, item creations, or modifications applied during a job.
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

// JobApprovalRequest models human review requests for high-risk job execution stages.
type JobApprovalRequest struct {
	ApprovalID          string               `json:"approval_id"`
	JobID               string               `json:"job_id"`
	PlanID              string               `json:"plan_id"`
	Status              string               `json:"status"`
	RequestedOperations []appv1.JobOperation `json:"requested_operations,omitempty"`
	Reason              string               `json:"reason,omitempty"`
	RiskTier            string               `json:"risk_tier,omitempty"`
	RequestedBy         string               `json:"requested_by,omitempty"`
	ReviewedBy          string               `json:"reviewed_by,omitempty"`
	RequestedAt         string               `json:"requested_at,omitempty"`
	ReviewedAt          string               `json:"reviewed_at,omitempty"`
}

// ExecutePlanRequest parameters define document and workspace identifiers needed for plan execution.
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

// Validate ensures that required JobID, DocumentID, and WorkspaceID fields are populated.
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

// JobPlanningSignals supplies context regarding existing items and aliases to guide LLM planning.
type JobPlanningSignals struct {
	DocumentID            string `json:"document_id,omitempty"`
	WorkspaceID           string `json:"workspace_id,omitempty"`
	SameDocumentItemCount int    `json:"same_document_item_count,omitempty"`
	ApprovedAliasCount    int    `json:"approved_alias_count,omitempty"`
	ProtectedAliasCount   int    `json:"protected_alias_count,omitempty"`
}

// JobEvaluationResult contains quality scoring and validation findings for job outputs.
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

// ActionStatus represents the status of an approval or review request.
type ActionStatus string

const (
	// ActionStatusPending indicates an action awaiting resolution.
	ActionStatusPending ActionStatus = "pending"
	// ActionStatusCompleted indicates that the action finished.
	ActionStatusCompleted ActionStatus = "completed"
	// ActionStatusRejected indicates that the action was rejected.
	ActionStatusRejected ActionStatus = "rejected"
	// ActionStatusFailed indicates that the action encountered an error.
	ActionStatusFailed ActionStatus = "failed"
)

// DefaultJobCapability constructs the baseline execution capabilities and safety ceilings for a new processing job.
// Must stay byte-for-byte in sync with the API copy in apps/api/internal/domain/job.go.
func DefaultJobCapability(jobID, workspaceID, documentID string, createdAt time.Time) *JobCapability {
	return &JobCapability{
		CapabilityID:       "cap_" + jobID,
		JobID:              jobID,
		WorkspaceID:        workspaceID,
		AllowedDocumentIDs: []string{documentID},
		AllowedOperations: []appv1.JobOperation{
			appv1.JobOperation_JOB_OPERATION_READ_TREE,
			appv1.JobOperation_JOB_OPERATION_READ_DOCUMENT,
			appv1.JobOperation_JOB_OPERATION_CREATE_ITEM,
			appv1.JobOperation_JOB_OPERATION_UPDATE_ITEM,
			appv1.JobOperation_JOB_OPERATION_INVOKE_LLM,
		},
		MaxLLMCalls:      128,
		MaxToolRuns:      0,
		MaxItemCreations: 4096,
		// Bound below MaxLLMCalls: each create_transform consumes LLM calls, so
		// 64 creations is already generous. Runs are higher because one created
		// transform executes per item.
		MaxTransformCreations: 64,
		MaxTransformRuns:      512,
		ExpiresAt:             createdAt.Add(24 * time.Hour).UTC().Format(time.RFC3339),
		CreatedAt:             createdAt.UTC().Format(time.RFC3339),
	}
}
