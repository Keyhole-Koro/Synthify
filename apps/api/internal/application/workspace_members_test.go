package application

import (
	"context"
	"errors"
	"strings"
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
		// invitee と other は、サインインしたアカウントでこのアドレスを確認済み。
		VerifiedEmails: verifiedEmailsByUser{"invitee": "invitee@example.com", "other": "other@example.com"},
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

// UpdateMemberRole only changes the role of someone who is already a member of
// the workspace. For anyone else it returns ErrNotFound and must not add them.
func TestUpdateMemberRole_TargetNotAMember_ReturnsErrNotFound(t *testing.T) {
	tests := []struct {
		name   string
		target string
		setup  func(t *testing.T, ctx context.Context, store *mock.Store)
	}{
		{name: "registered user who was never invited", target: "invitee"},
		{
			name:   "member of another workspace",
			target: "other-member",
			setup: func(t *testing.T, ctx context.Context, store *mock.Store) {
				account, err := store.GetOrCreateAccount(ctx, "other-owner")
				require.NoError(t, err)
				other, err := store.CreateWorkspace(ctx, account.AccountID, "other-workspace")
				require.NoError(t, err)
				_, err = store.UpsertUser(ctx, &domain.User{UserID: "other-member", Email: "other-member@example.com"})
				require.NoError(t, err)
				require.NoError(t, store.UpsertWorkspaceMember(ctx, other.WorkspaceID, "other-member", domain.WorkspaceRoleEditor, "other-owner"))
			},
		},
		{name: "unknown user", target: "ghost"},
		{name: "the owner", target: "owner"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			svc, store, wsID, _ := newMemberTestService(t)
			if tt.setup != nil {
				tt.setup(t, ctx, store)
			}
			roleBefore, err := store.GetWorkspaceRole(ctx, wsID, tt.target)
			require.NoError(t, err)

			_, err = svc.UpdateMemberRole(ctx, wsID, "owner", tt.target, domain.WorkspaceRoleEditor)
			assert.ErrorIs(t, err, domain.ErrNotFound)

			roleAfter, err := store.GetWorkspaceRole(ctx, wsID, tt.target)
			require.NoError(t, err)
			assert.Equal(t, roleBefore, roleAfter, "role in the workspace")
			members, err := store.ListWorkspaceMembers(ctx, wsID)
			require.NoError(t, err)
			for _, member := range members {
				assert.NotEqual(t, tt.target, member.UserID, "must not be added as a member")
			}
		})
	}
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

// verifiedEmailsByUser は VerifiedEmails の fake。user ごとに、サインインした
// アカウントで確認済みのアドレスを持つ。
type verifiedEmailsByUser map[string]string

func (v verifiedEmailsByUser) HasVerifiedEmail(_ context.Context, userID, email string) (bool, error) {
	verified, ok := v[userID]
	return ok && strings.EqualFold(verified, email), nil
}

type failingVerifiedEmails struct{ err error }

func (f failingVerifiedEmails) HasVerifiedEmail(context.Context, string, string) (bool, error) {
	return false, f.err
}

// 他人のアドレスでアカウントを作り、一度サインインしただけの squatter は、users に
// そのアドレスで保存されるが、確認はしていない。そのアドレスへの招待で squatter が
// メンバーになってはいけない。
func TestInviteMember_UnverifiedAddressOwner_IsNotInvited(t *testing.T) {
	ctx := context.Background()
	svc, store, wsID, _ := newMemberTestService(t)
	_, err := store.UpsertUser(ctx, &domain.User{UserID: "squatter", Email: "victim@example.com"})
	require.NoError(t, err, "UpsertUser squatter")

	_, err = svc.InviteMember(ctx, wsID, "owner", "victim@example.com", domain.WorkspaceRoleEditor)
	assert.ErrorIs(t, err, domain.ErrNotFound, "an unverified address is treated as unregistered")
	_, err = svc.GetWorkspace(ctx, wsID, "squatter")
	assert.ErrorIs(t, err, domain.ErrForbidden, "the squatter must not become a member")
}

// users.email はサインインのときにしか更新されない。その後で別のアドレスに移って
// 確認したアカウントには、古いアドレスへの招待を渡さない。
func TestInviteMember_StaleStoredAddress_IsNotInvited(t *testing.T) {
	ctx := context.Background()
	svc, store, wsID, _ := newMemberTestService(t)
	_, err := store.UpsertUser(ctx, &domain.User{UserID: "mover", Email: "old@example.com"})
	require.NoError(t, err, "UpsertUser mover")
	svc.emails = verifiedEmailsByUser{"mover": "new@example.com"}

	_, err = svc.InviteMember(ctx, wsID, "owner", "old@example.com", domain.WorkspaceRoleViewer)
	assert.ErrorIs(t, err, domain.ErrNotFound)
	_, err = svc.GetWorkspace(ctx, wsID, "mover")
	assert.ErrorIs(t, err, domain.ErrForbidden)
}

// 確認ができないとき (Firebase のエラー、確認の手段が無い) は、誰も招待しない。
func TestInviteMember_UncheckableAddress_InvitesNobody(t *testing.T) {
	ctx := context.Background()
	errLookup := errors.New("firebase unavailable")
	for name, emails := range map[string]VerifiedEmails{
		"lookup error": failingVerifiedEmails{err: errLookup},
		"no verifier":  nil,
	} {
		t.Run(name, func(t *testing.T) {
			svc, _, wsID, inviteeID := newMemberTestService(t)
			svc.emails = emails

			_, err := svc.InviteMember(ctx, wsID, "owner", "invitee@example.com", domain.WorkspaceRoleViewer)
			require.Error(t, err)
			if emails != nil {
				assert.ErrorIs(t, err, errLookup)
			}
			_, err = svc.GetWorkspace(ctx, wsID, inviteeID)
			assert.ErrorIs(t, err, domain.ErrForbidden, "the invitee must not become a member")
		})
	}
}
