package postgres

// These tests run the deletion SQL against a real CockroachDB: what matters is
// the foreign-key cascades and the correlated subqueries, which sqlmock cannot
// check. They are skipped unless SYNTHIFY_TEST_DATABASE_DSN points at a
// database with db/migrations applied, e.g.
//
//	docker run -d -p 127.0.0.1:26257:26257 cockroachdb/cockroach:latest-v24.2 start-single-node --insecure
//	for f in db/migrations/*.up.sql; do cockroach sql --insecure --host=127.0.0.1:26257 -d synthify < "$f"; done
//	SYNTHIFY_TEST_DATABASE_DSN='postgres://root@127.0.0.1:26257/synthify?sslmode=disable' go test ./apps/api/internal/repository/postgres/
//
// Every test writes its own freshly generated IDs, so they share a database
// without seeing each other's rows.

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/internal/platform/database"
)

func openTestDatabaseStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("SYNTHIFY_TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("SYNTHIFY_TEST_DATABASE_DSN is not set")
	}
	store, err := NewStore(context.Background(), dsn, database.PoolConfig{}, nil, discardLogger())
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// dbFixture inserts rows for one test. Every ID it hands out is fresh.
type dbFixture struct {
	t     *testing.T
	store *Store
	now   time.Time
}

func (f dbFixture) exec(query string, args ...any) {
	f.t.Helper()
	_, err := f.store.db.ExecContext(context.Background(), query, args...)
	require.NoError(f.t, err, query)
}

func (f dbFixture) account(usedBytes int64) string {
	id := newID()
	f.exec(`INSERT INTO accounts (account_id, name, plan, storage_quota_bytes, storage_used_bytes, created_at, updated_at)
VALUES ($1, 'acct', 'free', 1000000, $2, $3, $3)`, id, usedBytes, f.now)
	return id
}

func (f dbFixture) workspace(accountID string) string {
	id := newID()
	f.exec(`INSERT INTO workspaces (workspace_id, account_id, name, created_at, updated_at) VALUES ($1, $2, 'ws', $3, $3)`, id, accountID, f.now)
	return id
}

// document inserts a document with one file, one chunk, one job (with a log
// line and a checkpoint) and an upload reservation in the given status.
func (f dbFixture) document(accountID, workspaceID, reservationStatus string, size int64, expiresAt time.Time) (docID, fileID, jobID string) {
	docID, fileID, jobID = newID(), newID(), newID()
	f.exec(`INSERT INTO documents (document_id, workspace_id, uploaded_by, filename, mime_type, file_size, created_at)
VALUES ($1, $2, 'user', 'a.pdf', 'application/pdf', $3, $4)`, docID, workspaceID, size, f.now)
	f.exec(`INSERT INTO document_files (file_id, document_id, path, mime_type, file_size, created_at)
VALUES ($1, $2, 'extracted/a.pdf', 'application/pdf', $3, $4)`, fileID, docID, size, f.now)
	f.exec(`INSERT INTO document_chunks (chunk_id, document_id, file_id, text) VALUES ($1, $2, $3, 'chunk text')`, newID(), docID, fileID)
	f.exec(`INSERT INTO upload_reservations (reservation_id, account_id, workspace_id, document_id, expected_size_bytes, actual_size_bytes, status, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5, $5, $6, $7, $8)`, newID(), accountID, workspaceID, docID, size, reservationStatus, expiresAt, f.now)
	f.exec(`INSERT INTO document_processing_jobs (job_id, document_id, workspace_id, job_type, status, created_at, updated_at)
VALUES ($1, $2, $3, 'process_document', 'succeeded', $4, $4)`, jobID, docID, workspaceID, f.now)
	f.exec(`INSERT INTO job_logs (id, job_id, workspace_id, document_id, level, event, message, created_at)
VALUES ($1, $2, $3, $4, 'info', 'job.done', 'done', $5)`, newID(), jobID, workspaceID, docID, f.now)
	f.exec(`INSERT INTO job_stage_checkpoints (job_id, stage, status, gcs_ref, updated_at)
VALUES ($1, 'extract', 'succeeded', '', $2)`, jobID, f.now)
	return docID, fileID, jobID
}

// item inserts a tree node citing the given (document, file) pairs.
func (f dbFixture) item(workspaceID, parentID string, crossDocument bool, sources ...[2]string) string {
	id := newID()
	f.exec(`INSERT INTO tree_items (id, workspace_id, parent_id, title, created_at, updated_at, cross_document)
VALUES ($1, $2, $3, 'node', $4, $4, $5)`, id, workspaceID, sql.NullString{String: parentID, Valid: parentID != ""}, f.now, crossDocument)
	for _, source := range sources {
		f.exec(`INSERT INTO item_sources (item_id, document_id, file_id, chunk_id) VALUES ($1, $2, $3, '')`, id, source[0], source[1])
	}
	return id
}

// mutation records that a processing job wrote to a tree node, the way the
// worker logs creating ("append"), merging into or revising a node.
func (f dbFixture) mutation(jobID, workspaceID, itemID, mutationType string) {
	f.exec(`INSERT INTO job_mutation_logs (mutation_id, job_id, workspace_id, target_type, target_id, mutation_type, created_at)
VALUES ($1, $2, $3, 'item', $4, $5, $6)`, newID(), jobID, workspaceID, itemID, mutationType, f.now)
}

func (f dbFixture) count(query string, args ...any) int {
	f.t.Helper()
	var n int
	require.NoError(f.t, f.store.db.QueryRowContext(context.Background(), query, args...).Scan(&n), query)
	return n
}

func (f dbFixture) parentOf(itemID string) string {
	f.t.Helper()
	var parent sql.NullString
	require.NoError(f.t, f.store.db.QueryRowContext(context.Background(), `SELECT parent_id FROM tree_items WHERE id = $1`, itemID).Scan(&parent))
	return parent.String
}

func (f dbFixture) crossDocument(itemID string) bool {
	f.t.Helper()
	var cross bool
	require.NoError(f.t, f.store.db.QueryRowContext(context.Background(), `SELECT cross_document FROM tree_items WHERE id = $1`, itemID).Scan(&cross))
	return cross
}

func TestDeleteDocumentRows_DB_DeletesOnlyNodesNoOtherDocumentContributedTo(t *testing.T) {
	store := openTestDatabaseStore(t)
	f := dbFixture{t: t, store: store, now: time.Now().UTC()}
	ctx := context.Background()

	account := f.account(1000)
	ws := f.workspace(account)
	doc1, file1, job1 := f.document(account, ws, "confirmed", 300, f.now.Add(-time.Hour))
	doc2, file2, job2 := f.document(account, ws, "confirmed", 200, f.now.Add(-time.Hour))
	doc3, file3, _ := f.document(account, ws, "confirmed", 100, f.now.Add(-time.Hour))
	by1, by2, by3 := [2]string{doc1, file1}, [2]string{doc2, file2}, [2]string{doc3, file3}

	// a (doc1)                 deleted
	//   b (doc1)               deleted
	//     c (doc2)             kept: a and b go, so it becomes a root node
	//   d (no source)          kept: made by hand, also becomes a root node
	// e (doc2)                 kept
	//   f (doc1)               deleted
	//     g (doc1, doc2)       kept: moves up to e, no longer cross-document
	//   h (doc1, doc2, doc3)   kept: still cites two documents
	//   p (made by doc1's job) deleted: no source row, but doc1's job created it
	//   q (doc1's job, merged by doc2's job) kept: no longer cross-document
	//   m (made by hand)       kept
	a := f.item(ws, "", false, by1)
	b := f.item(ws, a, false, by1)
	c := f.item(ws, b, false, by2)
	d := f.item(ws, a, false)
	e := f.item(ws, "", false, by2)
	fNode := f.item(ws, e, false, by1)
	g := f.item(ws, fNode, true, by1, by2)
	h := f.item(ws, e, true, by1, by2, by3)
	p := f.item(ws, e, false)
	f.mutation(job1, ws, p, "append")
	q := f.item(ws, e, true)
	f.mutation(job1, ws, q, "append")
	f.mutation(job2, ws, q, "merge")
	m := f.item(ws, e, false)

	deleted, err := store.DeleteDocumentRows(ctx, doc1)
	require.NoError(t, err)

	assert.Equal(t, 4, deleted)
	for _, gone := range []string{a, b, fNode, p} {
		assert.Zero(t, f.count(`SELECT count(*) FROM tree_items WHERE id = $1`, gone), "node %s should be deleted", gone)
	}
	assert.Equal(t, "", f.parentOf(c))
	assert.Equal(t, "", f.parentOf(d))
	assert.Equal(t, e, f.parentOf(g))
	assert.Equal(t, e, f.parentOf(h))
	assert.False(t, f.crossDocument(g))
	assert.True(t, f.crossDocument(h))
	assert.Equal(t, e, f.parentOf(q))
	assert.False(t, f.crossDocument(q))
	assert.Equal(t, e, f.parentOf(m))

	for _, table := range []string{"documents", "document_files", "document_chunks", "upload_reservations", "document_processing_jobs", "item_sources"} {
		assert.Zero(t, f.count(`SELECT count(*) FROM `+table+` WHERE document_id = $1`, doc1), "%s rows of the deleted document", table)
	}
	assert.Zero(t, f.count(`SELECT count(*) FROM job_logs WHERE job_id = $1`, job1))
	assert.Zero(t, f.count(`SELECT count(*) FROM job_stage_checkpoints WHERE job_id = $1`, job1))

	// The other documents and their sources are untouched.
	assert.Equal(t, 2, f.count(`SELECT count(*) FROM documents WHERE workspace_id = $1`, ws))
	assert.Equal(t, 4, f.count(`SELECT count(*) FROM item_sources WHERE document_id = $1`, doc2))
	assert.Equal(t, 1, f.count(`SELECT count(*) FROM item_sources WHERE document_id = $1`, doc3))

	// Only the deleted document's confirmed bytes are released.
	assert.Equal(t, 700, f.count(`SELECT storage_used_bytes FROM accounts WHERE account_id = $1`, account))
}

func TestDeleteDocumentRows_DB_UnconfirmedUploadReleasesNothing(t *testing.T) {
	store := openTestDatabaseStore(t)
	f := dbFixture{t: t, store: store, now: time.Now().UTC()}

	account := f.account(500)
	ws := f.workspace(account)
	doc, _, _ := f.document(account, ws, "expired", 300, f.now.Add(-time.Hour))

	deleted, err := store.DeleteDocumentRows(context.Background(), doc)
	require.NoError(t, err)

	assert.Zero(t, deleted)
	assert.Zero(t, f.count(`SELECT count(*) FROM documents WHERE document_id = $1`, doc))
	assert.Equal(t, 500, f.count(`SELECT storage_used_bytes FROM accounts WHERE account_id = $1`, account))
}

func TestDeleteDocumentRows_DB_UsageNeverGoesNegative(t *testing.T) {
	store := openTestDatabaseStore(t)
	f := dbFixture{t: t, store: store, now: time.Now().UTC()}

	account := f.account(100)
	ws := f.workspace(account)
	doc, _, _ := f.document(account, ws, "confirmed", 300, f.now.Add(-time.Hour))

	_, err := store.DeleteDocumentRows(context.Background(), doc)
	require.NoError(t, err)

	assert.Zero(t, f.count(`SELECT storage_used_bytes FROM accounts WHERE account_id = $1`, account))
}

func TestDeleteDocumentRows_DB_MissingDocument(t *testing.T) {
	store := openTestDatabaseStore(t)

	_, err := store.DeleteDocumentRows(context.Background(), newID())

	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestGetDocumentDeletionState_DB(t *testing.T) {
	store := openTestDatabaseStore(t)
	f := dbFixture{t: t, store: store, now: time.Now().UTC()}
	ctx := context.Background()

	account := f.account(0)
	ws := f.workspace(account)
	uploading, _, uploadingJob := f.document(account, ws, "reserved", 10, f.now.Add(10*time.Minute))
	abandoned, _, _ := f.document(account, ws, "reserved", 10, f.now.Add(-time.Minute))
	done, _, _ := f.document(account, ws, "confirmed", 10, f.now.Add(-time.Hour))

	state, err := store.GetDocumentDeletionState(ctx, uploading)
	require.NoError(t, err)
	assert.True(t, state.UploadPending)
	assert.Equal(t, []string{uploadingJob}, state.JobIDs)

	state, err = store.GetDocumentDeletionState(ctx, abandoned)
	require.NoError(t, err)
	assert.False(t, state.UploadPending, "an expired reservation no longer holds a usable upload URL")

	state, err = store.GetDocumentDeletionState(ctx, done)
	require.NoError(t, err)
	assert.False(t, state.UploadPending)

	_, err = store.GetDocumentDeletionState(ctx, newID())
	assert.ErrorIs(t, err, domain.ErrNotFound)
}
