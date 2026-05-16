package mock

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/packages/shared/domain"
)

func fixedNow() time.Time {
	return time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
}

func candidate(name, wsID string) *domain.DynamicTool {
	return &domain.DynamicTool{
		Name:              name,
		Scope:             domain.ToolScopeWorkspace,
		OriginWorkspaceID: wsID,
		OriginJobID:       "job_1",
		Description:       "desc",
		Language:          "python",
		Code:              "print(1)",
		IOSchema:          "{}",
		DeclaredTier:      "tier_1",
		FloorTier:         "tier_1",
		Status:            domain.ToolStatusCandidate,
		InputSample:       "in",
	}
}

// ── RecordCandidate ───────────────────────────────────────────────────────────

func TestRecordCandidate_AssignsToolIDAndVersion(t *testing.T) {
	store := NewStore()

	got, err := store.RecordCandidate(ctx, candidate("normalize", "ws_a"))
	require.NoError(t, err)

	assert.NotEmpty(t, got.ToolID, "store must assign a tool id")
	assert.Equal(t, 1, got.Version, "first version is 1")
	assert.Equal(t, domain.ToolStatusCandidate, got.Status)
}

func TestRecordCandidate_SameNameSameWorkspace_IncrementsVersion(t *testing.T) {
	store := NewStore()

	first, err := store.RecordCandidate(ctx, candidate("normalize", "ws_a"))
	require.NoError(t, err)
	second, err := store.RecordCandidate(ctx, candidate("normalize", "ws_a"))
	require.NoError(t, err)

	assert.Equal(t, 1, first.Version)
	assert.Equal(t, 2, second.Version, "same (scope,ws,name) re-record bumps version")
}

func TestRecordCandidate_SameNameDifferentWorkspace_IndependentVersions(t *testing.T) {
	store := NewStore()

	a, err := store.RecordCandidate(ctx, candidate("normalize", "ws_a"))
	require.NoError(t, err)
	b, err := store.RecordCandidate(ctx, candidate("normalize", "ws_b"))
	require.NoError(t, err)

	assert.Equal(t, 1, a.Version)
	assert.Equal(t, 1, b.Version, "different workspace is an independent version line")
}

// ── ListCandidates ────────────────────────────────────────────────────────────

func TestListCandidates_ReturnsOnlyCandidates_OldestFirst(t *testing.T) {
	store := NewStore()
	first, _ := store.RecordCandidate(ctx, candidate("a", "ws_a"))
	second, _ := store.RecordCandidate(ctx, candidate("b", "ws_a"))

	got, err := store.ListCandidates(ctx, 10)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, first.ToolID, got[0].ToolID, "oldest first")
	assert.Equal(t, second.ToolID, got[1].ToolID)
}

func TestListCandidates_ExcludesPromoted(t *testing.T) {
	store := NewStore()
	promoted, _ := store.RecordCandidate(ctx, candidate("a", "ws_a"))
	require.NoError(t, store.PromoteCandidate(ctx, promoted.ToolID, domain.ToolStatusActive, fixedNow()))
	store.RecordCandidate(ctx, candidate("b", "ws_a"))

	got, err := store.ListCandidates(ctx, 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "b", got[0].Name, "promoted tool is no longer a candidate")
}

func TestListCandidates_RespectsLimit(t *testing.T) {
	store := NewStore()
	store.RecordCandidate(ctx, candidate("a", "ws_a"))
	store.RecordCandidate(ctx, candidate("b", "ws_a"))
	store.RecordCandidate(ctx, candidate("c", "ws_a"))

	got, err := store.ListCandidates(ctx, 2)
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

// ── PromoteCandidate ──────────────────────────────────────────────────────────

func TestPromoteCandidate_CandidateToActive(t *testing.T) {
	store := NewStore()
	tool, _ := store.RecordCandidate(ctx, candidate("a", "ws_a"))

	require.NoError(t, store.PromoteCandidate(ctx, tool.ToolID, domain.ToolStatusActive, fixedNow()))

	active, err := store.ResolveActiveTools(ctx, "ws_a")
	require.NoError(t, err)
	require.Len(t, active, 1)
	assert.Equal(t, tool.ToolID, active[0].ToolID)
	require.NotNil(t, active[0].PromotedAt)
}

func TestPromoteCandidate_Tier3ToHeld_NotResolvable(t *testing.T) {
	store := NewStore()
	tool, _ := store.RecordCandidate(ctx, candidate("a", "ws_a"))

	require.NoError(t, store.PromoteCandidate(ctx, tool.ToolID, domain.ToolStatusHeld, fixedNow()))

	active, err := store.ResolveActiveTools(ctx, "ws_a")
	require.NoError(t, err)
	assert.Empty(t, active, "held tool must not resolve")
}

func TestPromoteCandidate_UnknownID_Errors(t *testing.T) {
	store := NewStore()
	err := store.PromoteCandidate(ctx, "missing", domain.ToolStatusActive, fixedNow())
	assert.Error(t, err)
}

// ── ResolveActiveTools (tenant isolation) ─────────────────────────────────────

func TestResolveActiveTools_DefaultDeny_OtherTenantHidden(t *testing.T) {
	store := NewStore()
	a, _ := store.RecordCandidate(ctx, candidate("a", "ws_a"))
	b, _ := store.RecordCandidate(ctx, candidate("b", "ws_b"))
	require.NoError(t, store.PromoteCandidate(ctx, a.ToolID, domain.ToolStatusActive, fixedNow()))
	require.NoError(t, store.PromoteCandidate(ctx, b.ToolID, domain.ToolStatusActive, fixedNow()))

	got, err := store.ResolveActiveTools(ctx, "ws_a")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "ws_a", got[0].OriginWorkspaceID, "ws_a must not see ws_b's tool")
}

func TestResolveActiveTools_GlobalVisibleEverywhere(t *testing.T) {
	store := NewStore()
	g, _ := store.RecordCandidate(ctx, candidate("shared", "ws_a"))
	require.NoError(t, store.PromoteCandidate(ctx, g.ToolID, domain.ToolStatusActive, fixedNow()))
	require.NoError(t, store.PromoteToGlobal(ctx, g.ToolID, "operator"))

	got, err := store.ResolveActiveTools(ctx, "ws_b")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, domain.ToolScopeGlobal, got[0].Scope, "global tool resolves in any workspace")
}

func TestResolveActiveTools_ExcludesNonActive(t *testing.T) {
	store := NewStore()
	tool, _ := store.RecordCandidate(ctx, candidate("a", "ws_a"))
	// still candidate, never promoted

	got, err := store.ResolveActiveTools(ctx, "ws_a")
	require.NoError(t, err)
	assert.Empty(t, got, "candidate is not resolvable")
	_ = tool
}

// ── SetStatus (kill switch) ───────────────────────────────────────────────────

func TestSetStatus_DisableRemovesFromResolution(t *testing.T) {
	store := NewStore()
	tool, _ := store.RecordCandidate(ctx, candidate("a", "ws_a"))
	require.NoError(t, store.PromoteCandidate(ctx, tool.ToolID, domain.ToolStatusActive, fixedNow()))

	require.NoError(t, store.SetStatus(ctx, tool.ToolID, domain.ToolStatusDisabled))

	got, err := store.ResolveActiveTools(ctx, "ws_a")
	require.NoError(t, err)
	assert.Empty(t, got, "disabled is a one-field kill switch")
}

// ── IncrementUseCount ─────────────────────────────────────────────────────────

func TestIncrementUseCount_Accumulates(t *testing.T) {
	store := NewStore()
	tool, _ := store.RecordCandidate(ctx, candidate("a", "ws_a"))

	require.NoError(t, store.IncrementUseCount(ctx, tool.ToolID))
	require.NoError(t, store.IncrementUseCount(ctx, tool.ToolID))

	got, err := store.ListCandidates(ctx, 1)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, 2, got[0].UseCount)
}
