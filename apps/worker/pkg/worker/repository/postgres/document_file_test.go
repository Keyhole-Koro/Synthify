package postgres

import (
	"context"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

// CreateDocumentFile is now an upsert on (document_id, path): it RETURNs the
// file_id, which on conflict is the existing row's id. The Store must surface
// that returned id (not the freshly generated one) so a re-registration — the
// deterministic extract pre-pass plus a later agent extract_text — resolves to a
// stable file_id instead of erroring on the unique index.
func TestCreateDocumentFile_ReturnsRowFileID(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := &Store{db: db, logger: discardLogger()}

	existingID := "existing-file-id"
	createdAt := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO document_files")).
		WillReturnRows(sqlmock.NewRows([]string{"file_id", "created_at"}).AddRow(existingID, createdAt))

	got, err := store.CreateDocumentFile(context.Background(), "doc-1", "extracted/a.txt", "text/plain", 10)
	require.NoError(t, err)
	require.Equal(t, existingID, got.FileID, "must return the file_id from RETURNING, not the generated one")
	require.NoError(t, mock.ExpectationsWereMet())
}
