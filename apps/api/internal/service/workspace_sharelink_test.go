package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/api/internal/domain"
)

func TestCreateShareLink_Owner_GeneratesToken(t *testing.T) {
	ctx := context.Background()
	svc, _, wsID, _ := newMemberTestService(t)

	link, err := svc.CreateShareLink(ctx, wsID, "owner", domain.WorkspaceRoleViewer, "")
	require.NoError(t, err, "CreateShareLink")
	assert.NotEmpty(t, link.Token, "token generated")
	assert.Equal(t, wsID, link.WorkspaceID)
	assert.Equal(t, domain.WorkspaceRoleViewer, link.Role)
	assert.Equal(t, "owner", link.CreatedBy)
}

func TestCreateShareLink_DefaultsToViewer(t *testing.T) {
	ctx := context.Background()
	svc, _, wsID, _ := newMemberTestService(t)

	link, err := svc.CreateShareLink(ctx, wsID, "owner", "", "")
	require.NoError(t, err)
	assert.Equal(t, domain.WorkspaceRoleViewer, link.Role, "empty role defaults to viewer")
}

func TestCreateShareLink_OwnerRole_ReturnsErrInvalidArgument(t *testing.T) {
	ctx := context.Background()
	svc, _, wsID, _ := newMemberTestService(t)

	_, err := svc.CreateShareLink(ctx, wsID, "owner", domain.WorkspaceRoleOwner, "")
	assert.ErrorIs(t, err, domain.ErrInvalidArgument, "owner role not assignable to a link")
}

func TestCreateShareLink_NonOwner_ReturnsErrForbidden(t *testing.T) {
	ctx := context.Background()
	svc, store, wsID, inviteeID := newMemberTestService(t)
	require.NoError(t, store.UpsertWorkspaceMember(ctx, wsID, inviteeID, domain.WorkspaceRoleEditor, "owner"))

	_, err := svc.CreateShareLink(ctx, wsID, inviteeID, domain.WorkspaceRoleViewer, "")
	assert.ErrorIs(t, err, domain.ErrForbidden, "editor cannot create share links")
}

func TestCreateShareLink_TokensAreUnique(t *testing.T) {
	ctx := context.Background()
	svc, _, wsID, _ := newMemberTestService(t)

	a, err := svc.CreateShareLink(ctx, wsID, "owner", domain.WorkspaceRoleViewer, "")
	require.NoError(t, err)
	b, err := svc.CreateShareLink(ctx, wsID, "owner", domain.WorkspaceRoleViewer, "")
	require.NoError(t, err)
	assert.NotEqual(t, a.Token, b.Token, "each link gets a distinct token")
}

func TestResolveShareLink_Valid_ReturnsWorkspaceAndRole(t *testing.T) {
	ctx := context.Background()
	svc, _, wsID, _ := newMemberTestService(t)
	link, err := svc.CreateShareLink(ctx, wsID, "owner", domain.WorkspaceRoleViewer, "")
	require.NoError(t, err)

	ws, role, err := svc.ResolveShareLink(ctx, link.Token)
	require.NoError(t, err, "ResolveShareLink")
	assert.Equal(t, wsID, ws.WorkspaceID)
	assert.Equal(t, domain.WorkspaceRoleViewer, role)
}

func TestResolveShareLink_Unknown_ReturnsErrNotFound(t *testing.T) {
	ctx := context.Background()
	svc, _, _, _ := newMemberTestService(t)

	_, _, err := svc.ResolveShareLink(ctx, "no-such-token")
	assert.ErrorIs(t, err, domain.ErrNotFound, "unknown token")
}

func TestResolveShareLink_Revoked_ReturnsErrNotFound(t *testing.T) {
	ctx := context.Background()
	svc, _, wsID, _ := newMemberTestService(t)
	link, err := svc.CreateShareLink(ctx, wsID, "owner", domain.WorkspaceRoleViewer, "")
	require.NoError(t, err)
	require.NoError(t, svc.RevokeShareLink(ctx, wsID, "owner", link.Token))

	_, _, err = svc.ResolveShareLink(ctx, link.Token)
	assert.ErrorIs(t, err, domain.ErrNotFound, "revoked token")
}

func TestResolveShareLink_Expired_ReturnsErrNotFound(t *testing.T) {
	ctx := context.Background()
	svc, _, wsID, _ := newMemberTestService(t)
	past := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	link, err := svc.CreateShareLink(ctx, wsID, "owner", domain.WorkspaceRoleViewer, past)
	require.NoError(t, err)

	_, _, err = svc.ResolveShareLink(ctx, link.Token)
	assert.ErrorIs(t, err, domain.ErrNotFound, "expired token")
}

func TestRevokeShareLink_NonOwner_ReturnsErrForbidden(t *testing.T) {
	ctx := context.Background()
	svc, store, wsID, inviteeID := newMemberTestService(t)
	link, err := svc.CreateShareLink(ctx, wsID, "owner", domain.WorkspaceRoleViewer, "")
	require.NoError(t, err)
	require.NoError(t, store.UpsertWorkspaceMember(ctx, wsID, inviteeID, domain.WorkspaceRoleEditor, "owner"))

	err = svc.RevokeShareLink(ctx, wsID, inviteeID, link.Token)
	assert.ErrorIs(t, err, domain.ErrForbidden, "editor cannot revoke")
}

func TestListShareLinks_NonOwner_ReturnsErrForbidden(t *testing.T) {
	ctx := context.Background()
	svc, _, wsID, _ := newMemberTestService(t)

	_, err := svc.ListShareLinks(ctx, wsID, "stranger")
	assert.ErrorIs(t, err, domain.ErrForbidden, "non-owner cannot list links")
}
