package postgres

import (
	"context"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/api/internal/domain"
)

func newDynamicToolCandidate() *domain.DynamicTool {
	return &domain.DynamicTool{
		Name:              "normalize",
		Scope:             domain.ToolScopeWorkspace,
		OriginWorkspaceID: "ws_a",
		OriginJobID:       "job_1",
		Description:       "desc",
		Language:          "python",
		Code:              "print(1)",
		IOSchema:          "{}",
		DeclaredTier:      "tier_1",
		FloorTier:         "tier_1",
		RiskTier:          "tier_1",
		InputSample:       "in",
	}
}

func dynamicToolRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"tool_id", "name", "version", "scope", "origin_workspace_id", "origin_job_id",
		"description", "language", "code", "io_schema",
		"declared_tier", "floor_tier", "risk_tier", "status",
		"input_sample", "runtime_version", "use_count", "created_at", "promoted_at", "reviewed_by",
	})
}

// ── RecordCandidate ───────────────────────────────────────────────────────────

func TestRecordCandidate_AssignsVersionFromMaxAndInserts(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := &Store{db: db}
	ctx := context.Background()
	tool := newDynamicToolCandidate()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(MAX(version), 0)::int AS max_version")).
		WithArgs(string(tool.Scope), tool.OriginWorkspaceID, tool.Name).
		WillReturnRows(sqlmock.NewRows([]string{"max_version"}).AddRow(int32(2)))

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO dynamic_tools")).
		WithArgs(
			sqlmock.AnyArg(), // tool_id
			tool.Name, int32(3), string(tool.Scope), tool.OriginWorkspaceID, tool.OriginJobID,
			tool.Description, tool.Language, tool.Code, tool.IOSchema,
			tool.DeclaredTier, tool.FloorTier, tool.RiskTier,
			string(domain.ToolStatusCandidate), tool.InputSample, sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	got, err := store.RecordCandidate(ctx, tool)
	require.NoError(t, err)
	assert.Equal(t, 3, got.Version, "version is MAX+1")
	assert.NotEmpty(t, got.ToolID)
	assert.Equal(t, domain.ToolStatusCandidate, got.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── ListCandidates ────────────────────────────────────────────────────────────

func TestListCandidates_ReturnsCandidatesOldestFirst(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := &Store{db: db}
	ctx := context.Background()
	now := time.Now()

	rows := dynamicToolRows().
		AddRow("t1", "a", int32(1), "workspace", "ws_a", "job_1", "d", "python", "c", "{}",
			"tier_1", "tier_1", "tier_1", "candidate", "in", "", int32(0), now, nil, "").
		AddRow("t2", "b", int32(1), "workspace", "ws_a", "job_2", "d", "starlark", "c", "{}",
			"tier_2", "tier_2", "tier_2", "candidate", "in", "", int32(3), now, nil, "")

	mock.ExpectQuery(regexp.QuoteMeta("WHERE status = 'candidate'")).
		WithArgs(int32(10)).
		WillReturnRows(rows)

	got, err := store.ListCandidates(ctx, 10)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "t1", got[0].ToolID)
	assert.Equal(t, 3, got[1].UseCount)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── PromoteCandidate ──────────────────────────────────────────────────────────

func TestPromoteCandidate_UpdatesStatusAndPromotedAt(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := &Store{db: db}
	ctx := context.Background()
	at := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)

	mock.ExpectExec(regexp.QuoteMeta("UPDATE dynamic_tools\nSET status = $2, promoted_at = $3")).
		WithArgs("t1", string(domain.ToolStatusActive), at).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = store.PromoteCandidate(ctx, "t1", domain.ToolStatusActive, at)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPromoteCandidate_UnknownID_ReturnsNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := &Store{db: db}
	ctx := context.Background()
	at := time.Now()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE dynamic_tools\nSET status = $2, promoted_at = $3")).
		WithArgs("missing", string(domain.ToolStatusActive), at).
		WillReturnResult(sqlmock.NewResult(0, 0)) // 0 rows affected

	err = store.PromoteCandidate(ctx, "missing", domain.ToolStatusActive, at)
	assert.ErrorIs(t, err, domain.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── PromoteToGlobal ───────────────────────────────────────────────────────────

func TestPromoteToGlobal_SetsScopeAndReviewer(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := &Store{db: db}
	ctx := context.Background()

	mock.ExpectExec(regexp.QuoteMeta("SET scope = 'global', reviewed_by = $2")).
		WithArgs("t1", "operator").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = store.PromoteToGlobal(ctx, "t1", "operator")
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── SetStatus (kill switch) ───────────────────────────────────────────────────

func TestSetStatus_UpdatesStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := &Store{db: db}
	ctx := context.Background()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE dynamic_tools\nSET status = $2\nWHERE tool_id = $1")).
		WithArgs("t1", string(domain.ToolStatusDisabled)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = store.SetStatus(ctx, "t1", domain.ToolStatusDisabled)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── ResolveActiveTools (tenant isolation) ─────────────────────────────────────

func TestResolveActiveTools_QueriesWorkspaceAndGlobalActiveOnly(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := &Store{db: db}
	ctx := context.Background()
	now := time.Now()

	rows := dynamicToolRows().
		AddRow("t1", "a", int32(1), "workspace", "ws_a", "job_1", "d", "python", "c", "{}",
			"tier_1", "tier_1", "tier_1", "active", "in", "", int32(0), now, now, "")

	mock.ExpectQuery(regexp.QuoteMeta("WHERE status = 'active' AND (scope = 'global' OR origin_workspace_id = $1)")).
		WithArgs("ws_a").
		WillReturnRows(rows)

	got, err := store.ResolveActiveTools(ctx, "ws_a")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "ws_a", got[0].OriginWorkspaceID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── IncrementUseCount ─────────────────────────────────────────────────────────

func TestIncrementUseCount_BumpsCounter(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := &Store{db: db}
	ctx := context.Background()

	mock.ExpectExec(regexp.QuoteMeta("SET use_count = use_count + 1")).
		WithArgs("t1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = store.IncrementUseCount(ctx, "t1")
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
