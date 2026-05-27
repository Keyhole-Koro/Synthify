package postgres

// ConfirmDocumentUpload と ExpireUploadReservations / LogJobEvent の sqlmock テスト。
// ConfirmDocumentUpload は分岐が多い (not_found / 既 confirmed / expired /
// size mismatch / quota exceeded / happy) ので branch ごとに 1 ケース置く。

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
	"github.com/synthify/backend/apps/worker/pkg/worker/repository"
	"github.com/synthify/backend/internal/platform/job/log"
)

// fakeUploadURLIssuer は CreateDocument のテスト用スタブ。
// 固定の DocumentUploadTarget を返す。
type fakeUploadURLIssuer struct {
	target repository.DocumentUploadTarget
	err    error
}

func (f fakeUploadURLIssuer) IssueDocumentUploadURL(_ context.Context, _, _, _ string) (repository.DocumentUploadTarget, error) {
	return f.target, f.err
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newStoreWithMockForCreateDoc(t *testing.T, issuer repository.DocumentUploadURLIssuer) (*Store, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	store := &Store{db: db, uploadURLIssuer: issuer, logger: discardLogger()}
	return store, mock, func() { _ = db.Close() }
}

// lockWorkspaceAccountRows: lockWorkspaceAccount の SELECT 結果を構築する helper。
func lockWorkspaceAccountRows(accountID string, storageUsed, storageQuota, maxFileSize int64) *sqlmock.Rows {
	cols := []string{
		"account_id", "name", "plan",
		"storage_quota_bytes", "storage_used_bytes", "max_file_size_bytes",
		"max_uploads_per_5h", "max_uploads_per_1week",
		"stripe_customer_id", "stripe_subscription_id", "created_at",
	}
	return sqlmock.NewRows(cols).AddRow(
		accountID, "Acme", "free",
		storageQuota, storageUsed, maxFileSize,
		int32(20), int32(100),
		"", "", time.Now().UTC(),
	)
}

func reservationRows(accountID string, expectedSize int64, status string, expiresAt time.Time) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"account_id", "expected_size_bytes", "status", "expires_at"}).
		AddRow(accountID, expectedSize, status, expiresAt)
}

// ── ConfirmDocumentUpload ────────────────────────────────────────────────────

func TestConfirmDocumentUpload_NoReservation_ReturnsErrUploadNotConfirmed(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("FROM upload_reservations\nWHERE document_id = $1\nFOR UPDATE")).
		WithArgs("doc_missing").
		WillReturnError(errors.New("sql: no rows in result set"))
	mock.ExpectRollback()

	err := store.ConfirmDocumentUpload(context.Background(), "doc_missing", 100)
	assert.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConfirmDocumentUpload_AlreadyConfirmed_ReturnsNil(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("FOR UPDATE")).
		WithArgs("doc_1").
		WillReturnRows(reservationRows("acc_1", 100, "confirmed", time.Now().Add(time.Hour)))
	// nil を返すが deferred Rollback は走るので ExpectRollback を置く。
	mock.ExpectRollback()

	err := store.ConfirmDocumentUpload(context.Background(), "doc_1", 100)
	assert.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConfirmDocumentUpload_Expired_ReturnsErrUploadNotConfirmed(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("FOR UPDATE")).
		WithArgs("doc_1").
		WillReturnRows(reservationRows("acc_1", 100, "reserved", time.Now().Add(-time.Hour)))
	mock.ExpectRollback()

	err := store.ConfirmDocumentUpload(context.Background(), "doc_1", 100)
	assert.ErrorIs(t, err, domain.ErrUploadNotConfirmed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConfirmDocumentUpload_SizeMismatch_MarksFailedAndReturnsErr(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("FOR UPDATE")).
		WithArgs("doc_1").
		WillReturnRows(reservationRows("acc_1", 100, "reserved", time.Now().Add(time.Hour)))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE upload_reservations\nSET status = 'failed', actual_size_bytes = $2\nWHERE document_id = $1")).
		WithArgs("doc_1", int64(200)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectRollback()

	err := store.ConfirmDocumentUpload(context.Background(), "doc_1", 200)
	assert.ErrorIs(t, err, domain.ErrUploadSizeMismatch)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConfirmDocumentUpload_QuotaExceeded_ReturnsErr(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("FROM upload_reservations\nWHERE document_id = $1\nFOR UPDATE")).
		WithArgs("doc_1").
		WillReturnRows(reservationRows("acc_1", 100, "reserved", time.Now().Add(time.Hour)))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT storage_used_bytes, storage_quota_bytes\nFROM accounts\nWHERE account_id = $1\nFOR UPDATE")).
		WithArgs("acc_1").
		WillReturnRows(sqlmock.NewRows([]string{"storage_used_bytes", "storage_quota_bytes"}).
			AddRow(int64(950), int64(1000))) // 950 + 100 = 1050 > 1000
	mock.ExpectRollback()

	err := store.ConfirmDocumentUpload(context.Background(), "doc_1", 100)
	assert.ErrorIs(t, err, domain.ErrStorageQuotaExceeded)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConfirmDocumentUpload_Happy_CommitsAndReturnsNil(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("FROM upload_reservations\nWHERE document_id = $1\nFOR UPDATE")).
		WithArgs("doc_1").
		WillReturnRows(reservationRows("acc_1", 100, "reserved", time.Now().Add(time.Hour)))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT storage_used_bytes, storage_quota_bytes")).
		WithArgs("acc_1").
		WillReturnRows(sqlmock.NewRows([]string{"storage_used_bytes", "storage_quota_bytes"}).
			AddRow(int64(200), int64(1000))) // ok
	mock.ExpectExec(regexp.QuoteMeta("UPDATE accounts\nSET storage_used_bytes = storage_used_bytes + $2")).
		WithArgs("acc_1", int64(100), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE upload_reservations\nSET status = 'confirmed'")).
		WithArgs("doc_1", int64(100), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := store.ConfirmDocumentUpload(context.Background(), "doc_1", 100)
	assert.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

// ── ExpireUploadReservations ─────────────────────────────────────────────────

func TestExpireUploadReservations_ReturnsRowsAffected(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE upload_reservations\nSET status = 'expired'\nWHERE status = 'reserved' AND expires_at <= $1")).
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 3))

	n, err := store.ExpireUploadReservations(context.Background(), time.Now())
	require.NoError(t, err)
	assert.Equal(t, int64(3), n)
	require.NoError(t, mock.ExpectationsWereMet())
}

// ── CreateDocument ───────────────────────────────────────────────────────────

func TestCreateDocument_HappyPath_IssuesURLAndCommits(t *testing.T) {
	issuer := fakeUploadURLIssuer{
		target: repository.DocumentUploadTarget{URL: "https://gcs/u", Method: "PUT", ContentType: "application/pdf"},
	}
	store, mock, cleanup := newStoreWithMockForCreateDoc(t, issuer)
	defer cleanup()

	mock.ExpectBegin()
	// lockWorkspaceAccount: 生 SQL with FOR UPDATE OF a
	mock.ExpectQuery(regexp.QuoteMeta("FROM workspaces w\nJOIN accounts a ON a.account_id = w.account_id\nWHERE w.workspace_id = $1\nFOR UPDATE OF a")).
		WithArgs("ws_1").
		WillReturnRows(lockWorkspaceAccountRows("acc_1", 100, 10000, 1024*1024))
	// activeReservedBytes: SELECT COALESCE(SUM(...))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(SUM(expected_size_bytes), 0)")).
		WithArgs("acc_1", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(int64(0)))
	// sqlc CreateDocument
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO documents")).
		WithArgs(
			sqlmock.AnyArg(), // doc_id
			"ws_1",           // workspace_id
			"user_1",         // uploaded_by
			"paper.pdf",      // filename
			"application/pdf",
			int64(500),       // file_size
			sqlmock.AnyArg(), // created_at
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	// raw INSERT INTO upload_reservations
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO upload_reservations")).
		WithArgs(
			sqlmock.AnyArg(), // reservation_id
			"acc_1",
			"ws_1",
			sqlmock.AnyArg(), // doc_id
			int64(500),
			sqlmock.AnyArg(), // expires_at
			sqlmock.AnyArg(), // created_at
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	doc, target, err := store.CreateDocument(context.Background(), "ws_1", "user_1", "paper.pdf", "application/pdf", 500)
	require.NoError(t, err)
	require.NotNil(t, doc)
	assert.Equal(t, "paper.pdf", doc.Filename)
	assert.Equal(t, int64(500), doc.FileSize)
	assert.Equal(t, "https://gcs/u", target.URL)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateDocument_FileTooLarge_ReturnsErrFileTooLarge(t *testing.T) {
	store, mock, cleanup := newStoreWithMockForCreateDoc(t, fakeUploadURLIssuer{})
	defer cleanup()

	mock.ExpectBegin()
	// MaxFileSizeBytes=100 にして fileSize=500 にすれば File Too Large
	mock.ExpectQuery(regexp.QuoteMeta("FOR UPDATE OF a")).
		WithArgs("ws_1").
		WillReturnRows(lockWorkspaceAccountRows("acc_1", 0, 10000, 100))
	mock.ExpectRollback()

	_, _, err := store.CreateDocument(context.Background(), "ws_1", "user_1", "big.pdf", "application/pdf", 500)
	assert.ErrorIs(t, err, domain.ErrFileTooLarge)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateDocument_QuotaExceededByReservations_ReturnsErr(t *testing.T) {
	store, mock, cleanup := newStoreWithMockForCreateDoc(t, fakeUploadURLIssuer{})
	defer cleanup()

	mock.ExpectBegin()
	// used=900, quota=1000。reserved=200 を返せば 900+200+500=1600 > 1000
	mock.ExpectQuery(regexp.QuoteMeta("FOR UPDATE OF a")).
		WithArgs("ws_1").
		WillReturnRows(lockWorkspaceAccountRows("acc_1", 900, 1000, 1024*1024))
	// validateUploadSize 内でも storage check が走るが、その時 reserved は 0 として扱われる。
	// 900+500=1400 > 1000 で先に validateUploadSize 内で ErrStorageQuotaExceeded が返る。
	mock.ExpectRollback()

	_, _, err := store.CreateDocument(context.Background(), "ws_1", "user_1", "x.pdf", "application/pdf", 500)
	assert.ErrorIs(t, err, domain.ErrStorageQuotaExceeded)
	require.NoError(t, mock.ExpectationsWereMet())
}

// ── LogJobEvent ──────────────────────────────────────────────────────────────

func TestLogJobEvent_InsertsRowWithExpectedArgs(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO job_logs")).
		WithArgs(
			sqlmock.AnyArg(),     // id (newID())
			"job_1",              // job_id
			"ws_1",               // workspace_id
			"doc_1",              // document_id
			sqlmock.AnyArg(),     // created_at (nowTime())
			"INFO",               // level
			"job.started",        // event
			"job started",        // message
			sqlmock.AnyArg(),     // detail_json (serialized)
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := store.LogJobEvent(context.Background(), joblog.Event{
		JobID:       "job_1",
		WorkspaceID: "ws_1",
		DocumentID:  "doc_1",
		Level:       joblog.INFO,
		Event:       "job.started",
		Message:     "job started",
		Detail:      map[string]any{"k": "v"},
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
