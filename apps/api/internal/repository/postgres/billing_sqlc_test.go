package postgres

// このファイルは生 SQL → sqlc 化のときに導入された billing / account 系
// クエリの「パラメータ順 / SQL 形状 / 戻り値の意味」が崩れていないかを
// sqlmock で抑えるための回帰テストを集めている。
// 完全な動作保証ではなく「sqlc generate のパラメータ並びが変わったら気付ける」
// レベルの safety net が目的。

import (
	"context"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/api/internal/domain"
)

func newMockStore(t *testing.T) (*Store, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err, "sqlmock.New")
	return &Store{db: db}, mock, func() { _ = db.Close() }
}

// ── GetAccount ────────────────────────────────────────────────────────────────

func TestGetAccount_NotFound_ReturnsErrNotFound(t *testing.T) {
	store, mock, cleanup := newMockStore(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT account_id, name, plan.*FROM accounts.*WHERE account_id = \$1`).
		WithArgs("acc_unknown").
		WillReturnError(errors.New("sql: no rows in result set"))

	// sqlmock の WillReturnError は raw error しか伝えないので、別経路でテスト:
	_, err := store.GetAccount(context.Background(), "acc_unknown")
	assert.Error(t, err, "expected error")
}

func TestGetAccount_Success_ReturnsAccount(t *testing.T) {
	store, mock, cleanup := newMockStore(t)
	defer cleanup()

	now := time.Now().UTC()
	mock.ExpectQuery(`SELECT account_id, name, plan.*FROM accounts.*WHERE account_id = \$1`).
		WithArgs("acc_1").
		WillReturnRows(sqlmock.NewRows([]string{
			"account_id", "name", "plan",
			"storage_quota_bytes", "storage_used_bytes", "max_file_size_bytes",
			"max_uploads_per_5h", "max_uploads_per_1week",
			"stripe_customer_id", "stripe_subscription_id",
			"billing_status", "stripe_price_id", "billing_currency",
			"billing_amount_minor", "billing_interval",
			"current_period_end", "cancel_at_period_end", "billing_updated_at",
			"created_at",
		}).AddRow(
			"acc_1", "Acme", "pro",
			int64(1000), int64(200), int64(100),
			int32(20), int32(100),
			"cus_1", "sub_1",
			"active", "price_1", "usd",
			int64(1000), "month",
			now, false, now,
			now,
		))

	got, err := store.GetAccount(context.Background(), "acc_1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "acc_1", got.AccountID)
	assert.Equal(t, "pro", got.Plan)
	assert.Equal(t, "cus_1", got.StripeCustomerID)
	assert.Equal(t, "active", got.BillingStatus)
	assert.Equal(t, int64(1000), got.BillingAmountMinor)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── SetAccountStripeCustomerID ────────────────────────────────────────────────

func TestSetAccountStripeCustomerID_NoRows_ReturnsErrNotFound(t *testing.T) {
	store, mock, cleanup := newMockStore(t)
	defer cleanup()

	mock.ExpectExec(`UPDATE accounts.*SET stripe_customer_id = \$2, updated_at = \$3.*WHERE account_id = \$1`).
		WithArgs("acc_1", "cus_x", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := store.SetAccountStripeCustomerID(context.Background(), "acc_1", "cus_x")
	assert.ErrorIs(t, err, domain.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSetAccountStripeCustomerID_RowsAffected_ReturnsNil(t *testing.T) {
	store, mock, cleanup := newMockStore(t)
	defer cleanup()

	mock.ExpectExec(`UPDATE accounts.*SET stripe_customer_id`).
		WithArgs("acc_1", "cus_x", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := store.SetAccountStripeCustomerID(context.Background(), "acc_1", "cus_x")
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── ListStripeLinkedAccounts ─────────────────────────────────────────────────

func TestListStripeLinkedAccounts_ReturnsAccounts(t *testing.T) {
	store, mock, cleanup := newMockStore(t)
	defer cleanup()

	now := time.Now().UTC()
	mock.ExpectQuery(`SELECT account_id, name, plan.*FROM accounts.*WHERE stripe_customer_id <> ''.*LIMIT \$1`).
		WithArgs(int32(50)).
		WillReturnRows(sqlmock.NewRows([]string{
			"account_id", "name", "plan",
			"storage_quota_bytes", "storage_used_bytes", "max_file_size_bytes",
			"max_uploads_per_5h", "max_uploads_per_1week",
			"stripe_customer_id", "stripe_subscription_id",
			"billing_status", "stripe_price_id", "billing_currency",
			"billing_amount_minor", "billing_interval",
			"current_period_end", "cancel_at_period_end", "billing_updated_at",
			"created_at",
		}).AddRow(
			"acc_1", "Acme", "pro",
			int64(1000), int64(200), int64(100),
			int32(20), int32(100),
			"cus_1", "sub_1",
			"active", "price_1", "usd",
			int64(1000), "month",
			now, false, now,
			now,
		))

	got, err := store.ListStripeLinkedAccounts(context.Background(), 50)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "cus_1", got[0].StripeCustomerID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── ApplyBillingPlan ─────────────────────────────────────────────────────────

func TestApplyBillingPlan_NoRows_ReturnsErrNotFound(t *testing.T) {
	store, mock, cleanup := newMockStore(t)
	defer cleanup()

	// パラメータ順は sqlc 生成: plan, quota, max_file_size, per_5h, per_1week,
	// stripe_customer_id, stripe_subscription_id, now, account_id
	mock.ExpectExec(`UPDATE accounts.*SET plan`).
		WithArgs(
			"usage_based",          // plan
			sqlmock.AnyArg(),       // storage_quota_bytes (limits dependent)
			sqlmock.AnyArg(),       // max_file_size_bytes
			sqlmock.AnyArg(),       // max_uploads_per_5h
			sqlmock.AnyArg(),       // max_uploads_per_1week
			"cus_x",                // stripe_customer_id
			"sub_x",                // stripe_subscription_id
			sqlmock.AnyArg(),       // now
			"acc_missing",          // account_id (WHERE)
		).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := store.ApplyBillingPlan(context.Background(), "acc_missing", "cus_x", "sub_x", domain.BillingPlanUsageBased)
	assert.ErrorIs(t, err, domain.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyBillingPlanByStripeCustomerID_EmptyCustomerID_ReturnsErrNotFound_NoDBCall(t *testing.T) {
	store, _, cleanup := newMockStore(t)
	defer cleanup()

	// 空文字 customer ID は DB 呼ばずに即 ErrNotFound (early return)。
	err := store.ApplyBillingPlanByStripeCustomerID(context.Background(), "", "sub_x", domain.BillingPlanUsageBased)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// ── RecordBillingWebhookEvent ────────────────────────────────────────────────

func TestRecordBillingWebhookEvent_New_ReturnsTrue(t *testing.T) {
	store, mock, cleanup := newMockStore(t)
	defer cleanup()

	mock.ExpectExec(`INSERT INTO billing_events.*ON CONFLICT \(provider, event_id\) DO NOTHING`).
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
	assert.NoError(t, err)
	assert.True(t, ok, "new event should report inserted=true")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRecordBillingWebhookEvent_Duplicate_ReturnsFalse(t *testing.T) {
	store, mock, cleanup := newMockStore(t)
	defer cleanup()

	mock.ExpectExec(`INSERT INTO billing_events`).
		WithArgs("stripe", "evt_1", "invoice.paid", sqlmock.AnyArg(), "acc_1", "cus_1", "sub_1").
		WillReturnResult(sqlmock.NewResult(0, 0))

	ok, err := store.RecordBillingWebhookEvent(context.Background(), &domain.ProviderWebhookEvent{
		Provider:               "stripe",
		EventID:                "evt_1",
		EventType:              "invoice.paid",
		AccountID:              "acc_1",
		ExternalCustomerID:     "cus_1",
		ExternalSubscriptionID: "sub_1",
	})
	assert.NoError(t, err)
	assert.False(t, ok, "duplicate should report inserted=false")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── MarkBillingWebhookEventProcessed ─────────────────────────────────────────

func TestMarkBillingWebhookEventProcessed_HappyPath(t *testing.T) {
	store, mock, cleanup := newMockStore(t)
	defer cleanup()

	mock.ExpectExec(`UPDATE billing_events.*SET processing_status = \$3.*WHERE provider = \$1 AND event_id = \$2`).
		WithArgs("stripe", "evt_1", "processed", "", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := store.MarkBillingWebhookEventProcessed(context.Background(), "stripe", "evt_1", "processed", "")
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── ApplyBillingEvent ────────────────────────────────────────────────────────

func TestApplyBillingEvent_AccountIDPath_UsesByAccountQuery(t *testing.T) {
	store, mock, cleanup := newMockStore(t)
	defer cleanup()

	// account_id 経路: WHERE account_id = @account_id
	mock.ExpectExec(`UPDATE accounts.*WHERE account_id = \$\d+`).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := store.ApplyBillingEvent(context.Background(), &domain.ProviderWebhookEvent{
		AccountID:              "acc_1",
		ExternalCustomerID:     "cus_1",
		ExternalSubscriptionID: "sub_1",
		Plan:                   domain.BillingPlanUsageBased,
		Currency:               domain.BillingCurrencyUSD,
		Interval:               domain.BillingIntervalMonth,
		AmountMinor:             2000,
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyBillingEvent_StripeCustomerPath_UsesByStripeCustomerQuery(t *testing.T) {
	store, mock, cleanup := newMockStore(t)
	defer cleanup()

	// stripe_customer_id 経路 (account_id 未指定): WHERE stripe_customer_id = @stripe_customer_id_match
	mock.ExpectExec(`UPDATE accounts.*WHERE stripe_customer_id = \$\d+`).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := store.ApplyBillingEvent(context.Background(), &domain.ProviderWebhookEvent{
		AccountID:              "", // → ByStripeCustomer path
		ExternalCustomerID:     "cus_1",
		ExternalSubscriptionID: "sub_1",
		Plan:                   domain.BillingPlanUsageBased,
		Currency:               domain.BillingCurrencyUSD,
		Interval:               domain.BillingIntervalMonth,
		AmountMinor:             2000,
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyBillingEvent_NoRows_ReturnsErrNotFound(t *testing.T) {
	store, mock, cleanup := newMockStore(t)
	defer cleanup()

	mock.ExpectExec(`UPDATE accounts.*WHERE account_id = \$\d+`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := store.ApplyBillingEvent(context.Background(), &domain.ProviderWebhookEvent{
		AccountID: "acc_missing",
		Plan:      domain.BillingPlanUsageBased,
	})
	assert.ErrorIs(t, err, domain.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}
