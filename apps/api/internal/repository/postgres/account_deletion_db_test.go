package postgres

// Runs the account deletion SQL against CockroachDB; see
// data_deletion_db_test.go for how to point SYNTHIFY_TEST_DATABASE_DSN at one.

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/api/internal/domain"
)

func (f dbFixture) user() string {
	id := newID()
	f.exec(`INSERT INTO users (user_id, email, display_name, created_at, last_login_at, updated_at)
VALUES ($1, $1 || '@example.test', 'name', $2, $2, $2)`, id, f.now)
	return id
}

func (f dbFixture) member(accountID, userID string) {
	f.exec(`INSERT INTO account_users (account_id, user_id, role, joined_at) VALUES ($1, $2, 'owner', $3)`, accountID, userID, f.now)
}

func TestAccountDeletion_DB(t *testing.T) {
	store := openTestDatabaseStore(t)
	f := dbFixture{t: t, store: store, now: time.Now().UTC()}
	ctx := context.Background()

	// The user to delete: an account of their own with two workspaces, one
	// already deleted, plus usage and credit records.
	u := f.user()
	own := f.account(0)
	f.member(own, u)
	live := f.workspace(own)
	_, liveJob := f.populateWorkspace(own, live)
	earlier := f.workspace(own)
	f.markDeleted(earlier, f.now.Add(-time.Hour))
	f.exec(`INSERT INTO usage_events (event_id, account_id, model, created_at) VALUES ($1, $2, 'm', $3)`, newID(), own, f.now)
	f.exec(`INSERT INTO account_credits (credit_id, account_id, credit_type, amount_minor, granted_at) VALUES ($1, $2, 'free', 100, $3)`, newID(), own, f.now)

	// Someone else's workspace the user edits, and an account the two share.
	v := f.user()
	theirs := f.account(0)
	f.member(theirs, v)
	shared := f.workspace(theirs)
	theirDoc, _, theirJob := f.document(theirs, shared, "confirmed", 10, f.now.Add(-time.Hour))
	f.exec(`UPDATE documents SET uploaded_by = $2 WHERE document_id = $1`, theirDoc, u)
	f.exec(`UPDATE document_processing_jobs SET requested_by = $2 WHERE job_id = $1`, theirJob, u)
	handMade := f.item(shared, "", false)
	f.exec(`UPDATE tree_items SET created_by = $2 WHERE id = $1`, handMade, u)
	f.exec(`INSERT INTO workspace_members (workspace_id, user_id, role, invited_by, invited_at) VALUES ($1, $2, 'editor', $3, $4)`, shared, u, v, f.now)
	both := f.account(0)
	f.member(both, u)
	f.member(both, v)

	jobsActiveSince := f.now.Add(-30 * time.Minute)
	plan, err := store.GetAccountDeletionPlan(ctx, u, jobsActiveSince, f.now)
	require.NoError(t, err)
	assert.Equal(t, []string{own}, plan.AccountIDs, "an account shared with others is not the user's to delete")
	assert.ElementsMatch(t, []string{live, earlier}, plan.WorkspaceIDs)
	assert.False(t, plan.ActiveSubscription)
	assert.False(t, plan.Busy)

	f.exec(`UPDATE accounts SET stripe_subscription_id = 'sub', billing_status = 'active' WHERE account_id = $1`, own)
	plan, err = store.GetAccountDeletionPlan(ctx, u, jobsActiveSince, f.now)
	require.NoError(t, err)
	assert.True(t, plan.ActiveSubscription)
	f.exec(`UPDATE accounts SET cancel_at_period_end = true WHERE account_id = $1`, own)
	plan, err = store.GetAccountDeletionPlan(ctx, u, jobsActiveSince, f.now)
	require.NoError(t, err)
	assert.False(t, plan.ActiveSubscription, "a plan set to end does not block")

	f.exec(`UPDATE document_processing_jobs SET status = 'running', updated_at = $2 WHERE job_id = $1`, liveJob, f.now)
	plan, err = store.GetAccountDeletionPlan(ctx, u, jobsActiveSince, f.now)
	require.NoError(t, err)
	assert.True(t, plan.Busy)
	plan, err = store.GetAccountDeletionPlan(ctx, u, f.now.Add(time.Minute), f.now)
	require.NoError(t, err)
	assert.False(t, plan.Busy, "a job silent since before jobsActiveSince is stuck, not running")

	require.NoError(t, store.MarkWorkspacesDeleted(ctx, plan.WorkspaceIDs, f.now))
	assert.Equal(t, 1, f.count(`SELECT count(*) FROM workspaces WHERE workspace_id = $1 AND deleted_at = $2`, live, f.now))
	assert.Equal(t, 1, f.count(`SELECT count(*) FROM workspaces WHERE workspace_id = $1 AND deleted_at < $2`, earlier, f.now),
		"an earlier deletion keeps its time")

	assert.ErrorIs(t, store.DeleteAccountRows(ctx, u), domain.ErrConflict, "workspaces must be purged first")
	for _, ws := range plan.WorkspaceIDs {
		purged, err := store.PurgeWorkspaceRows(ctx, ws)
		require.NoError(t, err)
		assert.True(t, purged)
	}
	require.NoError(t, store.DeleteAccountRows(ctx, u))

	assert.Zero(t, f.count(`SELECT count(*) FROM users WHERE user_id = $1`, u))
	assert.Zero(t, f.count(`SELECT count(*) FROM account_users WHERE user_id = $1`, u))
	assert.Zero(t, f.count(`SELECT count(*) FROM accounts WHERE account_id = $1`, own))
	assert.Zero(t, f.count(`SELECT count(*) FROM usage_events WHERE account_id = $1`, own))
	assert.Zero(t, f.count(`SELECT count(*) FROM account_credits WHERE account_id = $1`, own))
	assert.Zero(t, f.count(`SELECT count(*) FROM workspace_members WHERE user_id = $1`, u))
	assert.Zero(t, f.count(`SELECT count(*) FROM documents WHERE uploaded_by = $1`, u))
	assert.Zero(t, f.count(`SELECT count(*) FROM document_processing_jobs WHERE requested_by = $1`, u))
	assert.Zero(t, f.count(`SELECT count(*) FROM tree_items WHERE created_by = $1`, u))

	// What belongs to others stays.
	assert.Equal(t, 1, f.count(`SELECT count(*) FROM documents WHERE document_id = $1 AND uploaded_by = ''`, theirDoc),
		"a document uploaded to someone else's workspace stays there, without the user's ID")
	assert.Equal(t, 1, f.count(`SELECT count(*) FROM tree_items WHERE id = $1`, handMade))
	assert.Equal(t, 1, f.count(`SELECT count(*) FROM workspaces WHERE workspace_id = $1 AND deleted_at IS NULL`, shared))
	assert.Equal(t, 1, f.count(`SELECT count(*) FROM account_users WHERE account_id = $1 AND user_id = $2`, both, v))
	assert.Equal(t, 1, f.count(`SELECT count(*) FROM users WHERE user_id = $1`, v))

	require.NoError(t, store.DeleteAccountRows(ctx, u), "deleting again is a no-op")
}
