package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ── EffectiveRiskTier ─────────────────────────────────────────────────────────

func TestEffectiveRiskTier_TakesMaxOfFloorAndDeclared(t *testing.T) {
	cases := []struct {
		name     string
		floor    string
		declared string
		want     string
	}{
		{"declared higher than floor", "tier_1", "tier_3", "tier_3"},
		{"floor higher than declared", "tier_3", "tier_1", "tier_3"},
		{"equal tiers", "tier_2", "tier_2", "tier_2"},
		{"floor wins on under-report", "tier_2", "tier_1", "tier_2"},
		{"empty declared normalizes to tier_1", "tier_2", "", "tier_2"},
		{"both empty normalize to tier_1", "", "", "tier_1"},
		{"approval_required alias resolves to tier_3", "tier_1", "approval_required", "tier_3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tool := &DynamicTool{FloorTier: tc.floor, DeclaredTier: tc.declared}
			assert.Equal(t, tc.want, tool.EffectiveRiskTier())
		})
	}
}

// ── UnderReported ─────────────────────────────────────────────────────────────

func TestUnderReported_TrueWhenDeclaredBelowFloor(t *testing.T) {
	tool := &DynamicTool{FloorTier: "tier_3", DeclaredTier: "tier_1"}
	assert.True(t, tool.UnderReported(), "declared below analyzer floor must flag under-report")
}

func TestUnderReported_FalseWhenDeclaredAtOrAboveFloor(t *testing.T) {
	atFloor := &DynamicTool{FloorTier: "tier_2", DeclaredTier: "tier_2"}
	aboveFloor := &DynamicTool{FloorTier: "tier_1", DeclaredTier: "tier_3"}
	assert.False(t, atFloor.UnderReported(), "declared == floor is not under-reporting")
	assert.False(t, aboveFloor.UnderReported(), "declared > floor is not under-reporting")
}

// ── AutoPromotable ────────────────────────────────────────────────────────────

func TestAutoPromotable_Tier1And2AutoPromote_Tier3Held(t *testing.T) {
	tier1 := &DynamicTool{FloorTier: "tier_1", DeclaredTier: "tier_1"}
	tier2 := &DynamicTool{FloorTier: "tier_2", DeclaredTier: "tier_1"}
	tier3 := &DynamicTool{FloorTier: "tier_3", DeclaredTier: "tier_1"}

	assert.True(t, tier1.AutoPromotable(), "tier_1 auto-promotes")
	assert.True(t, tier2.AutoPromotable(), "tier_2 auto-promotes (with notification)")
	assert.False(t, tier3.AutoPromotable(), "tier_3 requires human review")
}

func TestAutoPromotable_UsesEffectiveTierNotDeclared(t *testing.T) {
	// LLM declared tier_1 but analyzer floor is tier_3 → must not auto-promote.
	tool := &DynamicTool{FloorTier: "tier_3", DeclaredTier: "tier_1"}
	assert.False(t, tool.AutoPromotable(), "effective tier (floor) governs, not declared")
}

// ── Resolvable ────────────────────────────────────────────────────────────────

func TestResolvable_WorkspaceScope_OnlyOwningWorkspace(t *testing.T) {
	tool := &DynamicTool{
		Status:            ToolStatusActive,
		Scope:             ToolScopeWorkspace,
		OriginWorkspaceID: "ws_a",
	}
	assert.True(t, tool.Resolvable("ws_a"), "owning workspace resolves")
	assert.False(t, tool.Resolvable("ws_b"), "other tenant must not resolve (default-deny)")
}

func TestResolvable_GlobalScope_AnyWorkspace(t *testing.T) {
	tool := &DynamicTool{
		Status:            ToolStatusActive,
		Scope:             ToolScopeGlobal,
		OriginWorkspaceID: "ws_a",
	}
	assert.True(t, tool.Resolvable("ws_a"))
	assert.True(t, tool.Resolvable("ws_b"), "global tools resolve in any workspace")
}

func TestResolvable_NonActiveStatus_NeverResolves(t *testing.T) {
	for _, st := range []ToolStatus{
		ToolStatusCandidate, ToolStatusHeld, ToolStatusRejected, ToolStatusDisabled,
	} {
		tool := &DynamicTool{
			Status:            st,
			Scope:             ToolScopeWorkspace,
			OriginWorkspaceID: "ws_a",
		}
		assert.False(t, tool.Resolvable("ws_a"), "status %s must not resolve", st)
	}
}

func TestResolvable_NilReceiver_False(t *testing.T) {
	var tool *DynamicTool
	assert.False(t, tool.Resolvable("ws_a"))
}
