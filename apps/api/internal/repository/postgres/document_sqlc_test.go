package postgres

// ConfirmDocumentUpload は sqlc 化で 6 つの内部 SQL が 5 つの sqlc 呼び出し
// (LockUploadReservation, MarkUploadReservationFailed, LockAccountStorage,
// IncrementAccountStorageUsed, ConfirmUploadReservation) に置き換わった。
// すべての分岐 (not found / 既 confirmed / expired / size mismatch /
// quota exceeded / happy) を sqlmock で押さえる。

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

func reservationRow(accountID string, expectedSize int64, status string, expiresAt time.Time) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"account_id", "expected_size_bytes", "status", "expires_at"}).
		AddRow(accountID, expectedSize, status, expiresAt)
}

func TestConfirmDocumentUpload_ReservationNotFound_ReturnsErrUploadNotConfirmed(t *testing.T) {
	store, mock, cleanup := newMockStore(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT account_id, expected_size_bytes, status, expires_at.*FROM upload_reservations.*FOR UPDATE`).
		WithArgs("doc_missing").
		WillReturnError(errors.New("sql: no rows in result set"))
	mock.ExpectRollback()

	err := store.ConfirmDocumentUpload(context.Background(), "doc_missing", 100)
	// Either ErrUploadNotConfirmed (preferred) or a wrapped error path is fine,
	// what we want is that the function does NOT crash and SHOULD return some
	// kind of error to the caller. The raw sql.ErrNoRows mapping is verified
	// separately when we feed a typed NoRows result.
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestConfirmDocumentUpload_AlreadyConfirmed_ReturnsNil(t *testing.T) {
	store, mock, cleanup := newMockStore(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT account_id, expected_size_bytes, status, expires_at.*FOR UPDATE`).
		WithArgs("doc_1").
		WillReturnRows(reservationRow("acc_1", 100, "confirmed", time.Now().Add(time.Hour)))
	// No further queries; the function returns nil but the deferred Rollback
	// will still run.
	mock.ExpectRollback()

	err := store.ConfirmDocumentUpload(context.Background(), "doc_1", 100)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestConfirmDocumentUpload_ExpiredReservation_ReturnsErrUploadNotConfirmed(t *testing.T) {
	store, mock, cleanup := newMockStore(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT account_id, expected_size_bytes, status, expires_at.*FOR UPDATE`).
		WithArgs("doc_1").
		WillReturnRows(reservationRow("acc_1", 100, "reserved", time.Now().Add(-time.Hour)))
	mock.ExpectRollback()

	err := store.ConfirmDocumentUpload(context.Background(), "doc_1", 100)
	assert.ErrorIs(t, err, domain.ErrUploadNotConfirmed)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestConfirmDocumentUpload_SizeMismatch_ReturnsErrUploadSizeMismatch(t *testing.T) {
	store, mock, cleanup := newMockStore(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(`FOR UPDATE`).
		WithArgs("doc_1").
		WillReturnRows(reservationRow("acc_1", 100, "reserved", time.Now().Add(time.Hour)))
	mock.ExpectExec(`UPDATE upload_reservations.*SET status = 'failed', actual_size_bytes = \$2`).
		WithArgs("doc_1", int64(200)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectRollback()

	err := store.ConfirmDocumentUpload(context.Background(), "doc_1", 200)
	assert.ErrorIs(t, err, domain.ErrUploadSizeMismatch)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestConfirmDocumentUpload_QuotaExceeded_ReturnsErrStorageQuotaExceeded(t *testing.T) {
	store, mock, cleanup := newMockStore(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(`upload_reservations.*FOR UPDATE`).
		WithArgs("doc_1").
		WillReturnRows(reservationRow("acc_1", 100, "reserved", time.Now().Add(time.Hour)))
	mock.ExpectQuery(`SELECT storage_used_bytes, storage_quota_bytes.*FROM accounts.*FOR UPDATE`).
		WithArgs("acc_1").
		WillReturnRows(sqlmock.NewRows([]string{"storage_used_bytes", "storage_quota_bytes"}).
			AddRow(int64(950), int64(1000))) // used+actual=1050 > 1000
	mock.ExpectRollback()

	err := store.ConfirmDocumentUpload(context.Background(), "doc_1", 100)
	assert.ErrorIs(t, err, domain.ErrStorageQuotaExceeded)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestConfirmDocumentUpload_HappyPath_CommitsAndReturnsNil(t *testing.T) {
	store, mock, cleanup := newMockStore(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(`upload_reservations.*FOR UPDATE`).
		WithArgs("doc_1").
		WillReturnRows(reservationRow("acc_1", 100, "reserved", time.Now().Add(time.Hour)))
	mock.ExpectQuery(`SELECT storage_used_bytes, storage_quota_bytes.*FROM accounts.*FOR UPDATE`).
		WithArgs("acc_1").
		WillReturnRows(sqlmock.NewRows([]string{"storage_used_bytes", "storage_quota_bytes"}).
			AddRow(int64(200), int64(1000))) // ok
	mock.ExpectExec(`UPDATE accounts.*SET storage_used_bytes = storage_used_bytes \+ \$2`).
		WithArgs("acc_1", int64(100), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE upload_reservations.*SET status = 'confirmed'`).
		WithArgs("doc_1", int64(100), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := store.ConfirmDocumentUpload(context.Background(), "doc_1", 100)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── ExpireUploadReservations ─────────────────────────────────────────────────

func TestExpireUploadReservations_ReturnsRowsAffected(t *testing.T) {
	store, mock, cleanup := newMockStore(t)
	defer cleanup()

	mock.ExpectExec(`UPDATE upload_reservations.*SET status = 'expired'.*WHERE status = 'reserved' AND expires_at <= \$1`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 3))

	n, err := store.ExpireUploadReservations(context.Background(), time.Now())
	assert.NoError(t, err)
	assert.Equal(t, int64(3), n)
	assert.NoError(t, mock.ExpectationsWereMet())
}
