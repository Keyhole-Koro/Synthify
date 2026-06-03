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
	"github.com/synthify/backend/apps/api/internal/domain"
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
	//    workspace itself is the tree root; nodes are inserted directly under
	//    it (parent_id IS NULL) when a document is processed.
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO workspaces")).
		WithArgs(
			sqlmock.AnyArg(), // wsID (newID)
			"acc_1",          // account_id
			"My WS",          // name
			sqlmock.AnyArg(), // created_at
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// 2) GetWorkspace read-back (with account JOIN). 失敗してもフォールバックで
	//    組み立てた *Workspace が返るが、ここでは happy path として行を返す。
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

// ── UpdateWorkspaceName ──────────────────────────────────────────────────────

func TestUpdateWorkspaceName_NoRows_ReturnsErrNotFound(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE workspaces")).
		WithArgs("ws_missing", "new name", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))

	ws, err := store.UpdateWorkspaceName(context.Background(), "ws_missing", "new name")
	assert.ErrorIs(t, err, domain.ErrNotFound)
	assert.Nil(t, ws)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateWorkspaceName_Updated_ReturnsWorkspace(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE workspaces")).
		WithArgs("ws_1", "new name", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// GetWorkspace で読み戻し
	now := time.Now().UTC()
	wsCols := []string{
		"workspace_id", "account_id", "name", "created_at",
		"plan", "storage_used_bytes", "storage_quota_bytes", "max_file_size_bytes",
		"max_uploads_per_5h", "max_uploads_per_1week",
	}
	mock.ExpectQuery(regexp.QuoteMeta("FROM workspaces w\nJOIN accounts a")).
		WithArgs("ws_1").
		WillReturnRows(sqlmock.NewRows(wsCols).AddRow(
			"ws_1", "acc_1", "new name", now,
			"free", int64(0), int64(1000), int64(100),
			int32(20), int32(100),
		))
	mock.ExpectQuery(regexp.QuoteMeta("FROM tree_items")).WillReturnError(errMockNoRows)

	ws, err := store.UpdateWorkspaceName(context.Background(), "ws_1", "new name")
	require.NoError(t, err)
	require.NotNil(t, ws)
	assert.Equal(t, "new name", ws.Name)
}

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

func TestSetAccountStripeCustomerID_NoRows_ReturnsErrNotFound(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE accounts\nSET stripe_customer_id = $2, updated_at = $3\nWHERE account_id = $1")).
		WithArgs("acc_missing", "cus_x", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := store.SetAccountStripeCustomerID(context.Background(), "acc_missing", "cus_x")
	assert.ErrorIs(t, err, domain.ErrNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSetAccountStripeCustomerID_RowAffected_ReturnsNil(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE accounts")).
		WithArgs("acc_1", "cus_x", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := store.SetAccountStripeCustomerID(context.Background(), "acc_1", "cus_x")
	assert.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

// ── ListStripeLinkedAccounts ─────────────────────────────────────────────────

func TestListStripeLinkedAccounts_LimitClamping_Default100(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	now := time.Now().UTC()
	mock.ExpectQuery(regexp.QuoteMeta("LIMIT $1")).
		WithArgs(100). // limit<=0 → default 100
		WillReturnRows(sqlmock.NewRows(accountRowCols()).AddRow(sampleAccountRow(now)...))

	got, err := store.ListStripeLinkedAccounts(context.Background(), 0)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "cus_1", got[0].StripeCustomerID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListStripeLinkedAccounts_PassesExplicitLimit(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta("LIMIT $1")).
		WithArgs(50).
		WillReturnRows(sqlmock.NewRows(accountRowCols()))

	got, err := store.ListStripeLinkedAccounts(context.Background(), 50)
	require.NoError(t, err)
	assert.Empty(t, got)
	require.NoError(t, mock.ExpectationsWereMet())
}

// ── ApplyBillingPlan ─────────────────────────────────────────────────────────

func TestApplyBillingPlan_PassesArgsInExpectedOrder(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	// SQL は $1=account_id, $2=plan, $3..$6=limits, $7=stripe_customer_id, $8=sub, $9=now
	mock.ExpectExec(regexp.QuoteMeta("UPDATE accounts\nSET plan = $2")).
		WithArgs(
			"acc_1",           // $1 account_id
			"free",            // $2 plan
			sqlmock.AnyArg(),  // $3 storage_quota_bytes (limits)
			sqlmock.AnyArg(),  // $4 max_file_size_bytes
			sqlmock.AnyArg(),  // $5 max_uploads_per_5h
			sqlmock.AnyArg(),  // $6 max_uploads_per_1week
			"cus_x",           // $7 stripe_customer_id
			"sub_x",           // $8 stripe_subscription_id
			sqlmock.AnyArg(),  // $9 now
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := store.ApplyBillingPlan(context.Background(), "acc_1", "cus_x", "sub_x", domain.BillingPlanFree)
	assert.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyBillingPlan_NoRows_ReturnsErrNotFound(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE accounts")).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := store.ApplyBillingPlan(context.Background(), "acc_missing", "cus_x", "sub_x", domain.BillingPlanFree)
	assert.ErrorIs(t, err, domain.ErrNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

// ── ApplyBillingPlanByStripeCustomerID ───────────────────────────────────────

func TestApplyBillingPlanByStripeCustomerID_EmptyCustomerID_ShortCircuit(t *testing.T) {
	store, _, cleanup := newStoreWithMock(t)
	defer cleanup()

	// 空文字 customer ID は DB に行かず即 ErrNotFound。
	// mock に期待を設定しないことで「DB が呼ばれない」ことを担保している。
	err := store.ApplyBillingPlanByStripeCustomerID(context.Background(), "", "sub_x", domain.BillingPlanFree)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestApplyBillingPlanByStripeCustomerID_PassesArgsInExpectedOrder(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE accounts\nSET plan = $2")).
		WithArgs(
			"cus_x",           // $1 stripe_customer_id
			"free",            // $2 plan
			sqlmock.AnyArg(),  // $3 quota
			sqlmock.AnyArg(),  // $4 max_file_size
			sqlmock.AnyArg(),  // $5 per_5h
			sqlmock.AnyArg(),  // $6 per_1week
			"sub_x",           // $7 stripe_subscription_id
			sqlmock.AnyArg(),  // $8 now
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := store.ApplyBillingPlanByStripeCustomerID(context.Background(), "cus_x", "sub_x", domain.BillingPlanFree)
	assert.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

// ── RecordBillingWebhookEvent ────────────────────────────────────────────────

func TestRecordBillingWebhookEvent_EmptyEventID_ReturnsTrueWithoutDB(t *testing.T) {
	store, _, cleanup := newStoreWithMock(t)
	defer cleanup()

	// EventID="" は早期 return で true。DB を呼ばないことを mock 未設定で担保。
	ok, err := store.RecordBillingWebhookEvent(context.Background(), &domain.ProviderWebhookEvent{})
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestRecordBillingWebhookEvent_New_ReturnsTrue(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO billing_events")).
		WithArgs("stripe", "evt_1", "invoice.paid", sqlmock.AnyArg(), "acc_1", "cus_1", "sub_1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	ok, err := store.RecordBillingWebhookEvent(context.Background(), &domain.ProviderWebhookEvent{
		Provider:               "stripe",
		EventID:                "evt_1",
		EventType:              "invoice.paid",
		AccountID:              "acc_1",
		ExternalCustomerID:     "cus_1",
		ExternalSubscriptionID: "sub_1",
	})
	require.NoError(t, err)
	assert.True(t, ok, "new event should be reported as inserted")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRecordBillingWebhookEvent_Duplicate_ReturnsFalse(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO billing_events")).
		WillReturnResult(sqlmock.NewResult(0, 0))

	ok, err := store.RecordBillingWebhookEvent(context.Background(), &domain.ProviderWebhookEvent{
		EventID: "evt_dup",
	})
	require.NoError(t, err)
	assert.False(t, ok, "duplicate event should be reported as not inserted")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRecordBillingWebhookEvent_DefaultsProviderToStripe(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	// Provider="" → "stripe" にデフォルトする挙動を WithArgs で確認。
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO billing_events")).
		WithArgs("stripe", "evt_1", "invoice.paid", sqlmock.AnyArg(), "", "", "").
		WillReturnResult(sqlmock.NewResult(0, 1))

	_, err := store.RecordBillingWebhookEvent(context.Background(), &domain.ProviderWebhookEvent{
		EventID:   "evt_1",
		EventType: "invoice.paid",
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

// ── MarkBillingWebhookEventProcessed ─────────────────────────────────────────

func TestMarkBillingWebhookEventProcessed_EmptyEventID_NoOp(t *testing.T) {
	store, _, cleanup := newStoreWithMock(t)
	defer cleanup()

	err := store.MarkBillingWebhookEventProcessed(context.Background(), "stripe", "", "processed", "")
	assert.NoError(t, err)
}

func TestMarkBillingWebhookEventProcessed_PassesArgsInExpectedOrder(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE billing_events")).
		WithArgs("stripe", "evt_1", "processed", "some error", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := store.MarkBillingWebhookEventProcessed(context.Background(), "stripe", "evt_1", "processed", "some error")
	assert.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

// ── ApplyBillingEvent ────────────────────────────────────────────────────────

func TestApplyBillingEvent_NilEvent_NoOp(t *testing.T) {
	store, _, cleanup := newStoreWithMock(t)
	defer cleanup()
	assert.NoError(t, store.ApplyBillingEvent(context.Background(), nil))
}

func TestApplyBillingEvent_AccountIDPath_PassesArgsInOrder(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	// account_id 経路: WHERE account_id = $1。
	// $1=AccountID, $2="" (placeholder for stripe_customer_id slot), $3=plan, ...
	mock.ExpectExec(regexp.QuoteMeta("WHERE account_id = $1")).
		WithArgs(
			"acc_1",           // $1 account_id
			"",                // $2 stripe_customer_id slot (unused for this path)
			"free",            // $3 plan
			sqlmock.AnyArg(),  // $4 quota
			sqlmock.AnyArg(),  // $5 max_file_size
			sqlmock.AnyArg(),  // $6 per_5h
			sqlmock.AnyArg(),  // $7 per_1week
			"cus_1",           // $8 stripe_customer_id (CASE)
			"sub_1",           // $9 stripe_subscription_id
			sqlmock.AnyArg(),  // $10 billing_status
			"price_1",         // $11 stripe_price_id
			"usd",             // $12 currency
			int64(1000),       // $13 amount_minor
			"month",           // $14 interval
			sqlmock.AnyArg(),  // $15 current_period_end
			false,             // $16 cancel_at_period_end
			sqlmock.AnyArg(),  // $17 now
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := store.ApplyBillingEvent(context.Background(), &domain.ProviderWebhookEvent{
		AccountID:              "acc_1",
		ExternalCustomerID:     "cus_1",
		ExternalSubscriptionID: "sub_1",
		Plan:                   domain.BillingPlanFree,
		ExternalPriceID:        "price_1",
		Currency:               domain.BillingCurrencyUSD,
		AmountMinor:            1000,
		Interval:               domain.BillingIntervalMonth,
		CancelAtPeriodEnd:      false,
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyBillingEvent_StripeCustomerPath_PassesArgsInOrder(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	// stripe_customer_id 経路 (AccountID==""): WHERE stripe_customer_id = $2。
	// $1="" (placeholder), $2=customer_id, $3=plan, ..., $8=customer_id (CASE)
	mock.ExpectExec(regexp.QuoteMeta("WHERE stripe_customer_id = $2")).
		WithArgs(
			"",                // $1 account_id slot (unused)
			"cus_1",           // $2 stripe_customer_id (WHERE match)
			"free",            // $3 plan
			sqlmock.AnyArg(),  // $4-$7 limits
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			"cus_1",           // $8 stripe_customer_id (CASE passthru)
			"sub_1",           // $9 stripe_subscription_id
			sqlmock.AnyArg(),  // $10 status
			"price_1",         // $11
			"usd",             // $12
			int64(2000),       // $13
			"year",            // $14
			sqlmock.AnyArg(),  // $15
			true,              // $16
			sqlmock.AnyArg(),  // $17 now
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := store.ApplyBillingEvent(context.Background(), &domain.ProviderWebhookEvent{
		AccountID:              "", // → ByStripeCustomer path
		ExternalCustomerID:     "cus_1",
		ExternalSubscriptionID: "sub_1",
		Plan:                   domain.BillingPlanFree,
		ExternalPriceID:        "price_1",
		Currency:               domain.BillingCurrencyUSD,
		AmountMinor:            2000,
		Interval:               domain.BillingIntervalYear,
		CancelAtPeriodEnd:      true,
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyBillingEvent_NoRows_ReturnsErrNotFound(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE accounts")).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := store.ApplyBillingEvent(context.Background(), &domain.ProviderWebhookEvent{
		AccountID: "acc_missing",
		Plan:      domain.BillingPlanFree,
	})
	assert.ErrorIs(t, err, domain.ErrNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

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

// errMockNoRows は sqlmock で sql.ErrNoRows 相当を返すための代用エラー。
// repo 層は errors.Is(err, sql.ErrNoRows) を判定するため、ここでは
// 単に「クエリが空を返した」を表す任意のエラーで OK (RootItemID 空に倒れるだけ)。
var errMockNoRows = sqlmockNoRowsError{}

type sqlmockNoRowsError struct{}

func (sqlmockNoRowsError) Error() string { return "sql: no rows in result set" }
