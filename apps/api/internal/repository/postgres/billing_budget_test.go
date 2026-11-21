package postgres

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/internal/domain"
)

// RecordUsageAccounting compares the budget with the cost since the start of the
// current billing period, so last period's spend no longer counts.
func TestRecordUsageAccounting_BudgetUsesCurrentPeriod(t *testing.T) {
	periodStart := time.Now().UTC().Add(-10 * 24 * time.Hour).Truncate(time.Second)
	periodEnd := periodStart.Add(30 * 24 * time.Hour)
	cases := []struct {
		name         string
		periodMinor  int64
		wantExceeded bool
	}{
		{name: "this period is over budget", periodMinor: 1200, wantExceeded: true},
		{name: "this period is within budget", periodMinor: 800, wantExceeded: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store, mock, cleanup := newStoreWithMock(t)
			defer cleanup()

			mock.ExpectBegin()
			mock.ExpectExec(regexp.QuoteMeta("INSERT INTO usage_events")).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec(regexp.QuoteMeta("INSERT INTO account_usage_daily")).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery(regexp.QuoteMeta("RETURNING budget_limit_minor, current_period_started_at, current_period_end")).
				WithArgs("acc_1").
				WillReturnRows(sqlmock.NewRows([]string{"budget_limit_minor", "current_period_started_at", "current_period_end"}).
					AddRow(int64(1000), periodStart, periodEnd))
			// The sum starts at the Stripe period, not at the beginning of time.
			mock.ExpectQuery(regexp.QuoteMeta("WHERE account_id = $1 AND created_at >= $2")).
				WithArgs("acc_1", periodStart).
				WillReturnRows(sqlmock.NewRows([]string{"coalesce"}).AddRow(tc.periodMinor))
			mock.ExpectCommit()

			_, exceeded, err := store.RecordUsageAccounting(context.Background(), &domain.UsageEvent{
				EventID:   "ev_1",
				AccountID: "acc_1",
				Model:     "gemini",
				CostMinor: 100,
				Currency:  "jpy",
			}, time.Now().UTC().Format("2006-01-02"))
			require.NoError(t, err)
			assert.Equal(t, tc.wantExceeded, exceeded)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// Without a budget the period cost is not even queried.
func TestRecordUsageAccounting_NoBudgetSkipsSum(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO usage_events")).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO account_usage_daily")).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta("RETURNING budget_limit_minor")).
		WithArgs("acc_1").
		WillReturnRows(sqlmock.NewRows([]string{"budget_limit_minor", "current_period_started_at", "current_period_end"}).
			AddRow(int64(0), nil, nil))
	mock.ExpectCommit()

	_, exceeded, err := store.RecordUsageAccounting(context.Background(), &domain.UsageEvent{
		EventID: "ev_1", AccountID: "acc_1", Model: "gemini", CostMinor: 100, Currency: "jpy",
	}, time.Now().UTC().Format("2006-01-02"))
	require.NoError(t, err)
	assert.False(t, exceeded)
	require.NoError(t, mock.ExpectationsWereMet())
}
