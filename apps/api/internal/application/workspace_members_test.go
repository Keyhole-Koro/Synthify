package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository/mock"
)

// newMemberTestService は owner が所有する workspace と、招待先となる登録済み
// ユーザー (invitee) を用意した上で WorkspaceService を返す。
func newMemberTestService(t *testing.T) (*WorkspaceService, *mock.Store, string, string) {
	t.Helper()
	ctx := context.Background()
	store := mock.NewStore()
	wsID := mock.CreateUserWorkspaceFixture(t, ctx, store, "owner").Workspace.WorkspaceID
	_, err := store.UpsertUser(ctx, &domain.User{UserID: "invitee", Email: "invitee@example.com"})
	require.NoError(t, err, "UpsertUser invitee")
	svc := NewWorkspaceService(WorkspaceServiceDeps{
		Accounts:   store,
		Workspaces: store,
		Users:      store,
		Logger:     nil,
	})
	return svc, store, wsID, "invitee"
}

func TestInviteMember_Owner_AddsRegisteredUser(t *testing.T) {
	ctx := context.Background()
	svc, _, wsID, _ := newMemberTestService(t)

	member, err := svc.InviteMember(ctx, wsID, "owner", "invitee@example.com", domain.WorkspaceRoleEditor)
	require.NoError(t, err, "InviteMember")
	assert.Equal(t, "invitee", member.UserID, "member.UserID")
	assert.Equal(t, "invitee@example.com", member.Email, "member.Email")
	assert.Equal(t, domain.WorkspaceRoleEditor, member.Role, "member.Role")
	assert.Equal(t, "owner", member.InvitedBy, "member.InvitedBy")
}

func TestInviteMember_UnregisteredEmail_ReturnsErrNotFound(t *testing.T) {
	ctx := context.Background()
	svc, _, wsID, _ := newMemberTestService(t)

	_, err := svc.InviteMember(ctx, wsID, "owner", "stranger@example.com", domain.WorkspaceRoleViewer)
	assert.ErrorIs(t, err, domain.ErrNotFound, "InviteMember unregistered email")
}

func TestInviteMember_OwnerRole_ReturnsErrInvalidArgument(t *testing.T) {
	ctx := context.Background()
	svc, _, wsID, _ := newMemberTestService(t)

	// owner role は所有者 (account 経由) に予約されており、明示付与は不可。
	_, err := svc.InviteMember(ctx, wsID, "owner", "invitee@example.com", domain.WorkspaceRoleOwner)
	assert.ErrorIs(t, err, domain.ErrInvalidArgument, "InviteMember owner role")
}

func TestInviteMember_NonOwnerCaller_ReturnsErrForbidden(t *testing.T) {
	ctx := context.Background()
	svc, store, wsID, inviteeID := newMemberTestService(t)

	// invitee を editor として参加させた後、その editor が別ユーザーを招待しようとする。
	require.NoError(t, store.UpsertWorkspaceMember(ctx, wsID, inviteeID, domain.WorkspaceRoleEditor, "owner"))
	_, err := store.UpsertUser(ctx, &domain.User{UserID: "other", Email: "other@example.com"})
	require.NoError(t, err, "UpsertUser other")

	_, err = svc.InviteMember(ctx, wsID, inviteeID, "other@example.com", domain.WorkspaceRoleViewer)
	assert.ErrorIs(t, err, domain.ErrForbidden, "editor cannot invite")
}

func TestInviteMember_Stranger_ReturnsErrForbidden(t *testing.T) {
	ctx := context.Background()
	svc, _, wsID, _ := newMemberTestService(t)

	_, err := svc.InviteMember(ctx, wsID, "stranger", "invitee@example.com", domain.WorkspaceRoleViewer)
	assert.ErrorIs(t, err, domain.ErrForbidden, "non-member cannot invite")
}

// 招待された member が GetWorkspace にアクセスできる (共有が効く核心)。
func TestGetWorkspace_InvitedMember_HasAccess(t *testing.T) {
	ctx := context.Background()
	svc, _, wsID, inviteeID := newMemberTestService(t)

	// 招待前は非メンバーなので Forbidden。
	_, err := svc.GetWorkspace(ctx, wsID, inviteeID)
	require.ErrorIs(t, err, domain.ErrForbidden, "before invite")

	_, err = svc.InviteMember(ctx, wsID, "owner", "invitee@example.com", domain.WorkspaceRoleViewer)
	require.NoError(t, err, "InviteMember")

	got, err := svc.GetWorkspace(ctx, wsID, inviteeID)
	require.NoError(t, err, "after invite")
	assert.Equal(t, wsID, got.WorkspaceID, "workspace ID")
}

func TestUpdateMemberRole_Owner_UpdatesRole(t *testing.T) {
	ctx := context.Background()
	svc, store, wsID, inviteeID := newMemberTestService(t)
	require.NoError(t, store.UpsertWorkspaceMember(ctx, wsID, inviteeID, domain.WorkspaceRoleViewer, "owner"))

	member, err := svc.UpdateMemberRole(ctx, wsID, "owner", inviteeID, domain.WorkspaceRoleEditor)
	require.NoError(t, err, "UpdateMemberRole")
	assert.Equal(t, domain.WorkspaceRoleEditor, member.Role, "updated role")

	role, err := store.GetWorkspaceRole(ctx, wsID, inviteeID)
	require.NoError(t, err, "GetWorkspaceRole")
	assert.Equal(t, domain.WorkspaceRoleEditor, role, "persisted role")
}

func TestUpdateMemberRole_NonOwner_ReturnsErrForbidden(t *testing.T) {
	ctx := context.Background()
	svc, store, wsID, inviteeID := newMemberTestService(t)
	require.NoError(t, store.UpsertWorkspaceMember(ctx, wsID, inviteeID, domain.WorkspaceRoleEditor, "owner"))

	_, err := svc.UpdateMemberRole(ctx, wsID, inviteeID, inviteeID, domain.WorkspaceRoleViewer)
	assert.ErrorIs(t, err, domain.ErrForbidden, "editor cannot update roles")
}

func TestRemoveMember_Owner_RemovesMember(t *testing.T) {
	ctx := context.Background()
	svc, store, wsID, inviteeID := newMemberTestService(t)
	require.NoError(t, store.UpsertWorkspaceMember(ctx, wsID, inviteeID, domain.WorkspaceRoleViewer, "owner"))

	err := svc.RemoveMember(ctx, wsID, "owner", inviteeID)
	require.NoError(t, err, "RemoveMember")

	ok, err := store.IsWorkspaceAccessible(ctx, wsID, inviteeID)
	require.NoError(t, err, "IsWorkspaceAccessible")
	assert.False(t, ok, "removed member loses access")
}

func TestRemoveMember_NonOwner_ReturnsErrForbidden(t *testing.T) {
	ctx := context.Background()
	svc, store, wsID, inviteeID := newMemberTestService(t)
	require.NoError(t, store.UpsertWorkspaceMember(ctx, wsID, inviteeID, domain.WorkspaceRoleEditor, "owner"))

	err := svc.RemoveMember(ctx, wsID, inviteeID, inviteeID)
	assert.ErrorIs(t, err, domain.ErrForbidden, "editor cannot remove members")
}

func TestRemoveMember_Unknown_ReturnsErrNotFound(t *testing.T) {
	ctx := context.Background()
	svc, _, wsID, _ := newMemberTestService(t)

	err := svc.RemoveMember(ctx, wsID, "owner", "ghost")
	assert.ErrorIs(t, err, domain.ErrNotFound, "removing absent member")
}

func TestListMembers_NonMember_ReturnsErrForbidden(t *testing.T) {
	ctx := context.Background()
	svc, _, wsID, _ := newMemberTestService(t)

	_, err := svc.ListMembers(ctx, wsID, "stranger")
	assert.ErrorIs(t, err, domain.ErrForbidden, "non-member cannot list members")
}

// 招待された member の ListWorkspaces に被共有 workspace が出る (一覧導線が成立する)。
func TestListWorkspaces_InvitedMember_SeesSharedWorkspace(t *testing.T) {
	ctx := context.Background()
	svc, store, wsID, inviteeID := newMemberTestService(t)

	// 招待前は invitee の一覧に出ない。
	before, err := svc.ListWorkspaces(ctx, inviteeID)
	require.NoError(t, err, "ListWorkspaces before")
	assert.Empty(t, before, "non-member sees no workspaces")

	require.NoError(t, store.UpsertWorkspaceMember(ctx, wsID, inviteeID, domain.WorkspaceRoleViewer, "owner"))

	after, err := svc.ListWorkspaces(ctx, inviteeID)
	require.NoError(t, err, "ListWorkspaces after")
	require.Len(t, after, 1, "invited member sees the shared workspace")
	assert.Equal(t, wsID, after[0].WorkspaceID)
}
