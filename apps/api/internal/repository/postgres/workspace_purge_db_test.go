package postgres

// Runs the purge SQL against CockroachDB; see data_deletion_db_test.go for how
// to point SYNTHIFY_TEST_DATABASE_DSN at one.

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/api/internal/domain"
)

func (f dbFixture) markDeleted(workspaceID string, at time.Time) {
	f.exec(`UPDATE workspaces SET deleted_at = $2 WHERE workspace_id = $1`, workspaceID, at)
}

// populateWorkspace adds a row to every table that hangs off a workspace.
func (f dbFixture) populateWorkspace(accountID, workspaceID string) (docID, jobID string) {
	docID, fileID, jobID := f.document(accountID, workspaceID, "confirmed", 300, f.now.Add(-time.Hour))
	parent := f.item(workspaceID, "", false, [2]string{docID, fileID})
	child := f.item(workspaceID, parent, false, [2]string{docID, fileID})
	f.mutation(jobID, workspaceID, child, "append")
	f.exec(`INSERT INTO item_aliases (workspace_id, canonical_item_id, alias_item_id, updated_at) VALUES ($1, $2, $3, $4)`, workspaceID, parent, child, f.now)
	f.exec(`INSERT INTO workspace_members (workspace_id, user_id, role, invited_by, invited_at) VALUES ($1, 'member', 'editor', 'owner', $2)`, workspaceID, f.now)
	f.exec(`INSERT INTO workspace_share_links (token, workspace_id, created_by, created_at) VALUES ($1, $2, 'owner', $3)`, newID(), workspaceID, f.now)
	f.exec(`INSERT INTO dynamic_tools (tool_id, name, version, origin_workspace_id, origin_job_id, language, code, created_at)
VALUES ($1, 'tool', 1, $2, $3, 'starlark', 'def run(): pass', $4)`, newID(), workspaceID, jobID, f.now)
	return docID, jobID
}

// workspaceRowCounts counts the rows a workspace owns across every table that
// references it.
func (f dbFixture) workspaceRowCounts(workspaceID string) map[string]int {
	counts := map[string]int{}
	for _, table := range []string{
		"workspaces", "documents", "upload_reservations", "document_processing_jobs",
		"job_capabilities", "job_logs", "job_mutation_logs", "tree_items", "item_aliases",
		"workspace_members", "workspace_share_links",
	} {
		counts[table] = f.count(`SELECT count(*) FROM `+table+` WHERE workspace_id = $1`, workspaceID)
	}
	counts["dynamic_tools"] = f.count(`SELECT count(*) FROM dynamic_tools WHERE origin_workspace_id = $1`, workspaceID)
	counts["document_chunks"] = f.count(`SELECT count(*) FROM document_chunks c JOIN documents d ON d.document_id = c.document_id WHERE d.workspace_id = $1`, workspaceID)
	return counts
}

func TestWorkspacePurge_DB(t *testing.T) {
	store := openTestDatabaseStore(t)
	f := dbFixture{t: t, store: store, now: time.Now().UTC()}
	ctx := context.Background()

	account := f.account(1000)
	old := f.workspace(account)
	oldDoc, oldJob := f.populateWorkspace(account, old)
	f.markDeleted(old, f.now.Add(-40*24*time.Hour))
	recent := f.workspace(account)
	f.markDeleted(recent, f.now.Add(-5*24*time.Hour))
	live := f.workspace(account)
	f.populateWorkspace(account, live)

	cutoff := f.now.Add(-30 * 24 * time.Hour)
	listed, err := store.ListWorkspacesDeletedBefore(ctx, cutoff, 1000)
	require.NoError(t, err)
	assert.Contains(t, listed, old)
	assert.NotContains(t, listed, recent, "deleted within the retention period")
	assert.NotContains(t, listed, live)

	owners, err := store.GetWorkspaceStorageOwners(ctx, old)
	require.NoError(t, err)
	assert.Equal(t, []string{oldDoc}, owners.DocumentIDs)
	assert.Equal(t, []string{oldJob}, owners.JobIDs)
	_, err = store.GetWorkspaceStorageOwners(ctx, live)
	assert.ErrorIs(t, err, domain.ErrNotFound, "a live workspace is never a purge target")

	liveBefore := f.workspaceRowCounts(live)
	purged, err := store.PurgeWorkspaceRows(ctx, live)
	require.NoError(t, err)
	assert.False(t, purged)
	assert.Equal(t, liveBefore, f.workspaceRowCounts(live))

	purged, err = store.PurgeWorkspaceRows(ctx, old)
	require.NoError(t, err)
	assert.True(t, purged)
	for table, n := range f.workspaceRowCounts(old) {
		assert.Zero(t, n, "%s rows of the purged workspace", table)
	}
	assert.Equal(t, liveBefore, f.workspaceRowCounts(live), "the live workspace is untouched")
	assert.Equal(t, 700, f.count(`SELECT storage_used_bytes FROM accounts WHERE account_id = $1`, account),
		"the purged workspace's confirmed bytes go back to the account")

	purged, err = store.PurgeWorkspaceRows(ctx, old)
	require.NoError(t, err)
	assert.False(t, purged, "purging twice is a no-op")
}
