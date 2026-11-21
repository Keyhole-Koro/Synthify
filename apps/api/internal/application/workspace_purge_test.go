package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/api/internal/domain"
)

// purgeLog records, in order, the stored-file prefixes and the workspace rows
// a purge deletes, so tests can check that files go before rows.
type purgeLog struct {
	events []string
}

type fakeWorkspacePurgeRepo struct {
	log        *purgeLog
	listed     []string
	listErr    error
	listCutoff time.Time
	listLimit  int
	deleted    map[string]*domain.WorkspaceStorageOwners // workspaces marked deleted
}

func (r *fakeWorkspacePurgeRepo) ListWorkspacesDeletedBefore(_ context.Context, cutoff time.Time, limit int) ([]string, error) {
	r.listCutoff, r.listLimit = cutoff, limit
	return r.listed, r.listErr
}

func (r *fakeWorkspacePurgeRepo) GetWorkspaceStorageOwners(_ context.Context, workspaceID string) (*domain.WorkspaceStorageOwners, error) {
	owners, ok := r.deleted[workspaceID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return owners, nil
}

func (r *fakeWorkspacePurgeRepo) PurgeWorkspaceRows(_ context.Context, workspaceID string) (bool, error) {
	if _, ok := r.deleted[workspaceID]; !ok {
		return false, nil
	}
	delete(r.deleted, workspaceID)
	r.log.events = append(r.log.events, "rows "+workspaceID)
	return true, nil
}

type loggingPrefixDeleter struct {
	log    *purgeLog
	failOn string
}

func (d *loggingPrefixDeleter) DeleteObjectsWithPrefix(_ context.Context, prefix string) (int, error) {
	if prefix == d.failOn {
		return 0, errors.New("storage: permission denied")
	}
	d.log.events = append(d.log.events, "files "+prefix)
	return 1, nil
}

var purgeNow = time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)

func newTestWorkspacePurger(repo *fakeWorkspacePurgeRepo, objects *loggingPrefixDeleter) *WorkspacePurger {
	return NewWorkspacePurger(WorkspacePurgerDeps{
		Repo:      repo,
		Objects:   objects,
		Retention: 30 * 24 * time.Hour,
		Logger:    discardLogger(),
		Now:       func() time.Time { return purgeNow },
	})
}

func TestPurgeDeletedWorkspaces_DeletesFilesThenRowsOfWorkspacesPastRetention(t *testing.T) {
	log := &purgeLog{}
	repo := &fakeWorkspacePurgeRepo{
		log:    log,
		listed: []string{"ws1"},
		deleted: map[string]*domain.WorkspaceStorageOwners{
			"ws1": {DocumentIDs: []string{"doc1", "doc2"}, JobIDs: []string{"job1"}},
		},
	}
	purger := newTestWorkspacePurger(repo, &loggingPrefixDeleter{log: log})

	purged, err := purger.PurgeDeletedWorkspaces(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, purged)
	assert.Equal(t, purgeNow.Add(-30*24*time.Hour), repo.listCutoff)
	assert.Equal(t, defaultWorkspacePurgeBatch, repo.listLimit)
	assert.Equal(t, []string{
		"files .cache/v1/doc1/",
		"files .cache/v1/doc2/",
		"files .checkpoints/job1/",
		"files ws1/",
		"rows ws1",
	}, log.events)
}

// A workspace whose files cannot be deleted keeps its rows, so the next sweep
// still knows which files to delete; the other workspaces are purged anyway.
func TestPurgeDeletedWorkspaces_KeepsGoingPastAFailure(t *testing.T) {
	log := &purgeLog{}
	repo := &fakeWorkspacePurgeRepo{
		log:    log,
		listed: []string{"ws1", "ws2"},
		deleted: map[string]*domain.WorkspaceStorageOwners{
			"ws1": {},
			"ws2": {},
		},
	}
	purger := newTestWorkspacePurger(repo, &loggingPrefixDeleter{log: log, failOn: "ws1/"})

	purged, err := purger.PurgeDeletedWorkspaces(context.Background())

	require.Error(t, err)
	assert.Equal(t, 1, purged)
	assert.Contains(t, repo.deleted, "ws1", "ws1 stays marked deleted for the next sweep")
	assert.NotContains(t, repo.deleted, "ws2")
	assert.Equal(t, []string{"files ws2/", "rows ws2"}, log.events)
}

func TestPurgeWorkspace_LeavesWorkspacesNotMarkedDeletedAlone(t *testing.T) {
	log := &purgeLog{}
	repo := &fakeWorkspacePurgeRepo{log: log, deleted: map[string]*domain.WorkspaceStorageOwners{}}
	purger := newTestWorkspacePurger(repo, &loggingPrefixDeleter{log: log})

	err := purger.PurgeWorkspace(context.Background(), "live-or-gone")

	require.NoError(t, err)
	assert.Empty(t, log.events)
}

func TestPurgeDeletedWorkspaces_ReportsListFailure(t *testing.T) {
	log := &purgeLog{}
	repo := &fakeWorkspacePurgeRepo{log: log, listErr: errors.New("db down")}
	purger := newTestWorkspacePurger(repo, &loggingPrefixDeleter{log: log})

	purged, err := purger.PurgeDeletedWorkspaces(context.Background())

	require.Error(t, err)
	assert.Zero(t, purged)
}

func TestNewWorkspacePurger_HonoursBatchSize(t *testing.T) {
	repo := &fakeWorkspacePurgeRepo{log: &purgeLog{}}
	purger := NewWorkspacePurger(WorkspacePurgerDeps{Repo: repo, Objects: &loggingPrefixDeleter{log: &purgeLog{}}, BatchSize: 3})

	_, err := purger.PurgeDeletedWorkspaces(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 3, repo.listLimit)
}
