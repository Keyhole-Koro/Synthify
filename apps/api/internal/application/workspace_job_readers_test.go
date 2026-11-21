package application

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingReaders records what the workspace service mirrors into the
// Firestore reader documents.
type recordingReaders struct {
	mu         sync.Mutex
	grants     []string // "user@workspace"
	revokes    []string // "user@workspace"
	revokedAll []string
	err        error
}

func (r *recordingReaders) Grant(_ context.Context, userID string, workspaceIDs ...string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, ws := range workspaceIDs {
		r.grants = append(r.grants, userID+"@"+ws)
	}
	return r.err
}

func (r *recordingReaders) Revoke(_ context.Context, userID, workspaceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.revokes = append(r.revokes, userID+"@"+workspaceID)
	return r.err
}

func (r *recordingReaders) RevokeAll(_ context.Context, workspaceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.revokedAll = append(r.revokedAll, workspaceID)
	return r.err
}

func (r *recordingReaders) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.grants, r.revokes, r.revokedAll = nil, nil, nil
}

func sorted(s []string) []string {
	out := append([]string(nil), s...)
	sort.Strings(out)
	return out
}

// newReaderTestService is newMemberTestService (an owner's workspace and a
// registered invitee) with the reader mirror recorded.
func newReaderTestService(t *testing.T) (*WorkspaceService, string, *recordingReaders) {
	t.Helper()
	svc, _, wsID, _ := newMemberTestService(t)
	readers := &recordingReaders{}
	svc.jobReaders = readers
	return svc, wsID, readers
}

func TestJobStatusReaders_ListingGrantsEveryListedWorkspace(t *testing.T) {
	ctx := context.Background()
	svc, wsID, readers := newReaderTestService(t)
	second, err := svc.CreateWorkspace(ctx, "second", "owner")
	require.NoError(t, err)
	readers.reset()

	listed, err := svc.ListWorkspaces(ctx, "owner")
	require.NoError(t, err)
	require.Len(t, listed, 2)
	assert.Equal(t, sorted([]string{"owner@" + wsID, "owner@" + second.WorkspaceID}), sorted(readers.grants))

	readers.reset()
	listed, err = svc.ListWorkspaces(ctx, "stranger")
	require.NoError(t, err)
	assert.Empty(t, listed)
	assert.Empty(t, readers.grants, "a user with no workspace is granted nothing")
}

func TestJobStatusReaders_OpeningAWorkspaceGrantsOnlyWithAccess(t *testing.T) {
	ctx := context.Background()
	svc, wsID, readers := newReaderTestService(t)

	_, err := svc.GetWorkspace(ctx, wsID, "owner")
	require.NoError(t, err)
	assert.Equal(t, []string{"owner@" + wsID}, readers.grants)

	readers.reset()
	_, err = svc.GetWorkspace(ctx, wsID, "stranger")
	require.Error(t, err)
	assert.Empty(t, readers.grants, "no grant without access")
}

func TestJobStatusReaders_CreatingAWorkspaceGrantsTheCreator(t *testing.T) {
	ctx := context.Background()
	svc, _, readers := newReaderTestService(t)
	readers.reset()

	ws, err := svc.CreateWorkspace(ctx, "fresh", "owner")
	require.NoError(t, err)
	assert.Equal(t, []string{"owner@" + ws.WorkspaceID}, readers.grants)
}

func TestJobStatusReaders_InviteGrantsAndRemovalRevokes(t *testing.T) {
	ctx := context.Background()
	svc, wsID, readers := newReaderTestService(t)

	_, err := svc.InviteMember(ctx, wsID, "owner", "stranger@example.com", "viewer")
	require.Error(t, err, "an unregistered address is not invited")
	assert.Empty(t, readers.grants)

	_, err = svc.InviteMember(ctx, wsID, "owner", "invitee@example.com", "viewer")
	require.NoError(t, err)
	assert.Equal(t, []string{"invitee@" + wsID}, readers.grants)

	require.NoError(t, svc.RemoveMember(ctx, wsID, "owner", "invitee"))
	assert.Equal(t, []string{"invitee@" + wsID}, readers.revokes)
}

func TestJobStatusReaders_RemovalKeepsAccessThroughTheAccount(t *testing.T) {
	ctx := context.Background()
	svc, wsID, readers := newReaderTestService(t)
	// The owner reaches the workspace through its account, so removing an
	// extra membership row must not take the job status away.
	require.NoError(t, svc.workspaces.UpsertWorkspaceMember(ctx, wsID, "owner", "editor", "owner"))

	require.NoError(t, svc.RemoveMember(ctx, wsID, "owner", "owner"))
	assert.Empty(t, readers.revokes)
}

func TestJobStatusReaders_DeletingAWorkspaceRevokesEveryone(t *testing.T) {
	ctx := context.Background()
	svc, wsID, readers := newReaderTestService(t)

	require.Error(t, svc.DeleteWorkspace(ctx, wsID, "invitee"), "only the owner deletes")
	assert.Empty(t, readers.revokedAll)

	require.NoError(t, svc.DeleteWorkspace(ctx, wsID, "owner"))
	assert.Equal(t, []string{wsID}, readers.revokedAll)
}

func TestJobStatusReaders_MirrorFailuresDoNotFailTheCall(t *testing.T) {
	ctx := context.Background()
	svc, wsID, readers := newReaderTestService(t)
	readers.err = errors.New("firestore unavailable")

	_, err := svc.ListWorkspaces(ctx, "owner")
	assert.NoError(t, err)
	_, err = svc.GetWorkspace(ctx, wsID, "owner")
	assert.NoError(t, err)
	_, err = svc.InviteMember(ctx, wsID, "owner", "invitee@example.com", "editor")
	assert.NoError(t, err)
	assert.NoError(t, svc.RemoveMember(ctx, wsID, "owner", "invitee"))
	assert.NoError(t, svc.DeleteWorkspace(ctx, wsID, "owner"))
}
