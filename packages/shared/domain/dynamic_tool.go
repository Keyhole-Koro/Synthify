package domain

import (
	"time"

	"github.com/synthify/backend/packages/shared/util"
)

// ToolScope controls who may resolve a dynamic tool. Tools are born
// workspace-scoped; promotion to global is a separate explicit step.
// See docs/improvements/dynamic-tool-synthesis.md.
type ToolScope string

const (
	ToolScopeWorkspace ToolScope = "workspace"
	ToolScopeGlobal    ToolScope = "global"
)

// ToolStatus is the lifecycle state of a dynamic tool. The registry only
// loads `active` tools into the agent's tool set; any other state removes the
// tool from every job (so `disabled` is a one-field kill switch).
type ToolStatus string

const (
	ToolStatusCandidate ToolStatus = "candidate" // recorded, not yet promoted
	ToolStatusActive    ToolStatus = "active"    // promoted, resolvable
	ToolStatusHeld      ToolStatus = "held"      // awaiting human review (tier 3)
	ToolStatusRejected  ToolStatus = "rejected"  // review declined
	ToolStatusDisabled  ToolStatus = "disabled"  // emergency kill switch
)

// DynamicTool is transform code the LLM generated at runtime and that was
// recorded as a promotion candidate. The registry key is
// (Scope, OriginWorkspaceID, Name, Version); the LLM only ever refers to Name
// and the registry resolves it against the current job's workspace.
type DynamicTool struct {
	ToolID            string    `json:"tool_id"`
	Name              string    `json:"name"`
	Version           int       `json:"version"`
	Scope             ToolScope `json:"scope"`
	OriginWorkspaceID string    `json:"origin_workspace_id"`
	OriginJobID       string    `json:"origin_job_id"`

	Description string `json:"description"`
	Language    string `json:"language"` // python | sh | starlark
	Code        string `json:"code"`
	IOSchema    string `json:"io_schema"` // JSON Schema for input + output

	// DeclaredTier is the LLM self-report. FloorTier is the executor's static
	// analyzer lower bound. RiskTier is the effective value = max(floor, declared).
	// All three are stored so the declared-vs-machine gap stays auditable.
	DeclaredTier string `json:"declared_tier"`
	FloorTier    string `json:"floor_tier"`
	RiskTier     string `json:"risk_tier"`

	Status ToolStatus `json:"status"`

	// InputSample is the create_transform sanity-check input. Retaining it is
	// mandatory: without it neither the promotion gate nor regression eval can
	// reproduce why the tool was promoted.
	InputSample string `json:"input_sample"`

	// RuntimeVersion records the executor runtime at promotion time, since a
	// runtime change alters reproducibility.
	RuntimeVersion string `json:"runtime_version,omitempty"`

	UseCount   int        `json:"use_count"`
	CreatedAt  time.Time  `json:"created_at"`
	PromotedAt *time.Time `json:"promoted_at,omitempty"`
	ReviewedBy string     `json:"reviewed_by,omitempty"`
}

// EffectiveRiskTier returns max(FloorTier, DeclaredTier). The floor wins when
// the LLM under-reports; callers should log an audit event in that case.
func (t *DynamicTool) EffectiveRiskTier() string {
	floor := util.NormalizeRiskTier(t.FloorTier)
	declared := util.NormalizeRiskTier(t.DeclaredTier)
	if riskTierRank(floor) >= riskTierRank(declared) {
		return floor
	}
	return declared
}

// UnderReported is true when the LLM declared a lower tier than the analyzer
// floor. The tool is still tiered at the floor, but this signals an audit log.
func (t *DynamicTool) UnderReported() bool {
	return riskTierRank(util.NormalizeRiskTier(t.DeclaredTier)) <
		riskTierRank(util.NormalizeRiskTier(t.FloorTier))
}

// AutoPromotable reports whether this tool may be promoted without human
// review. tier_1/tier_2 auto-promote (tier_2 with notification); tier_3
// requires a human and stays held. Mirrors JobExecutionPlan.RequiresApproval.
func (t *DynamicTool) AutoPromotable() bool {
	return riskTierRank(t.EffectiveRiskTier()) < riskTierRank("tier_3")
}

// Resolvable reports whether the registry may load this tool for a job running
// in the given workspace. Tenant isolation is default-deny: only the owning
// workspace, or globally-promoted tools, resolve.
func (t *DynamicTool) Resolvable(workspaceID string) bool {
	if t == nil || t.Status != ToolStatusActive {
		return false
	}
	if t.Scope == ToolScopeGlobal {
		return true
	}
	return t.Scope == ToolScopeWorkspace && t.OriginWorkspaceID == workspaceID
}
