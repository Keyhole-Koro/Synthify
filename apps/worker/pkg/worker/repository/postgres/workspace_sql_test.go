package postgres

// workspace.go の billing / account / workspace 系生 SQL に対する sqlmock 回帰テスト。
// 「パラメータ順 / WHERE 句 / RowsAffected 解釈」が壊れたときに検出する safety net。
// document_test.go の TestCreateProcessingJobCreatesJobBeforeCapability と
// 同じスタイル (regexp.QuoteMeta + WithArgs) で書いている。

import (
	"context"
	"database/sql/driver"
	"errors"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newStoreWithMock(t *testing.T) (*Store, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	return &Store{db: db}, mock, func() { _ = db.Close() }
}

func accountRowCols() []string {
	return []string{
		"account_id", "name", "plan",
		"storage_quota_bytes", "storage_used_bytes", "max_file_size_bytes",
		"max_uploads_per_5h", "max_uploads_per_1week",
		"stripe_customer_id", "stripe_subscription_id",
		"billing_status", "stripe_price_id", "billing_currency",
		"billing_amount_minor", "billing_interval",
		"current_period_end", "cancel_at_period_end", "billing_updated_at",
		"created_at",
	}
}

func sampleAccountRow(now time.Time) []driver.Value {
	return []driver.Value{
		"acc_1", "Acme", "free",
		int64(1000), int64(200), int64(100),
		int32(20), int32(100),
		"cus_1", "sub_1",
		"active", "price_1", "usd",
		int64(2000), "month",
		now, false, now,
		now,
	}
}

// ── CreateWorkspace ──────────────────────────────────────────────────────────

func TestCreateWorkspace_InsertsWorkspace(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	// 1) workspace insert. No workspace_root tree_item is created — the
	//    workspace itself is the tree root.
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO workspaces")).
		WithArgs(
			sqlmock.AnyArg(), // wsID (newID)
			"acc_1",          // account_id
			"My WS",          // name
			sqlmock.AnyArg(), // created_at
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// 2) GetWorkspace read-back (with account JOIN).
	now := time.Now().UTC()
	wsCols := []string{
		"workspace_id", "account_id", "name", "created_at",
		"plan", "storage_used_bytes", "storage_quota_bytes", "max_file_size_bytes",
		"max_uploads_per_5h", "max_uploads_per_1week",
	}
	mock.ExpectQuery(regexp.QuoteMeta("FROM workspaces w\nJOIN accounts a")).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows(wsCols).AddRow(
			"ws_x", "acc_1", "My WS", now,
			"free", int64(0), int64(1000), int64(100),
			int32(20), int32(100),
		))

	ws, err := store.CreateWorkspace(context.Background(), "acc_1", "My WS")
	require.NoError(t, err)
	require.NotNil(t, ws)
	assert.Equal(t, "My WS", ws.Name)
}

func TestCreateWorkspace_WorkspaceInsertFails_ReturnsErr(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO workspaces")).
		WillReturnError(errors.New("fk violation"))

	ws, err := store.CreateWorkspace(context.Background(), "acc_missing", "x")
	assert.Error(t, err)
	assert.Nil(t, ws)
	require.NoError(t, mock.ExpectationsWereMet())
}

// NOTE: UpdateWorkspaceName は worker repo には実装されていない (api repo 専用)。
// 該当テストは api 側の workspace_sql_test.go にある。

// ── GetAccount ────────────────────────────────────────────────────────────────

func TestGetAccount_Found_ReturnsAccount(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	now := time.Now().UTC()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT account_id, name, plan")).
		WithArgs("acc_1").
		WillReturnRows(sqlmock.NewRows(accountRowCols()).AddRow(sampleAccountRow(now)...))

	got, err := store.GetAccount(context.Background(), "acc_1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "acc_1", got.AccountID)
	assert.Equal(t, "cus_1", got.StripeCustomerID)
	assert.Equal(t, "active", got.BillingStatus)
	assert.Equal(t, int64(2000), got.BillingAmountMinor)
	require.NoError(t, mock.ExpectationsWereMet())
}

// ── SetAccountStripeCustomerID ────────────────────────────────────────────────

// ── ListStripeLinkedAccounts ─────────────────────────────────────────────────

// ── ApplyBillingPlan ─────────────────────────────────────────────────────────

// ── ApplyBillingPlanByStripeCustomerID ───────────────────────────────────────

// ── RecordBillingWebhookEvent ────────────────────────────────────────────────

// ── MarkBillingWebhookEventProcessed ─────────────────────────────────────────

// ── ApplyBillingEvent ────────────────────────────────────────────────────────

// ── ListWorkspacesByUser ─────────────────────────────────────────────────────

func TestListWorkspacesByUser_ReturnsWorkspaces(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	now := time.Now().UTC()
	cols := []string{
		"workspace_id", "account_id", "name", "created_at",
		"plan", "storage_used_bytes", "storage_quota_bytes", "max_file_size_bytes",
		"max_uploads_per_5h", "max_uploads_per_1week",
	}
	mock.ExpectQuery(regexp.QuoteMeta("FROM workspaces w\nJOIN account_users au")).
		WithArgs("u_1").
		WillReturnRows(sqlmock.NewRows(cols).AddRow(
			"ws_1", "acc_1", "My Workspace", now,
			"free", int64(200), int64(1000), int64(100),
			int32(20), int32(100),
		))
	got, err := store.ListWorkspacesByUser(context.Background(), "u_1")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "ws_1", got[0].WorkspaceID)
	assert.Equal(t, "My Workspace", got[0].Name)
}

// ── GetWorkspace ─────────────────────────────────────────────────────────────

func TestGetWorkspace_Found_ReturnsWorkspace(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	now := time.Now().UTC()
	cols := []string{
		"workspace_id", "account_id", "name", "created_at",
		"plan", "storage_used_bytes", "storage_quota_bytes", "max_file_size_bytes",
		"max_uploads_per_5h", "max_uploads_per_1week",
	}
	mock.ExpectQuery(regexp.QuoteMeta("FROM workspaces w\nJOIN accounts a")).
		WithArgs("ws_1").
		WillReturnRows(sqlmock.NewRows(cols).AddRow(
			"ws_1", "acc_1", "My Workspace", now,
			"free", int64(0), int64(1000), int64(100),
			int32(20), int32(100),
		))

	got, err := store.GetWorkspace(context.Background(), "ws_1")
	require.NoError(t, err)
	assert.Equal(t, "ws_1", got.WorkspaceID)
	assert.Equal(t, "free", got.Plan)
}
