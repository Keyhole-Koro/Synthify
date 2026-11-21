package handler

import (
	"context"
	"testing"

	connect "connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/api/internal/application"
	"github.com/synthify/backend/apps/api/internal/auth"
	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository/mock"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

type recordingAuthUsers struct{ deleted []string }

func (r *recordingAuthUsers) DeleteUser(_ context.Context, uid string) error {
	r.deleted = append(r.deleted, uid)
	return nil
}

type nopWorkspacePurge struct{}

func (nopWorkspacePurge) PurgeWorkspace(context.Context, string) error { return nil }

func newDeletingUserHandler(store *mock.Store, authUsers *recordingAuthUsers) *UserHandler {
	return NewUserHandler(application.NewUserService(application.UserServiceDeps{Users: store, Accounts: store})).
		WithAccountDeletion(application.NewAccountDeletionService(application.AccountDeletionServiceDeps{
			Repo:       store,
			Workspaces: nopWorkspacePurge{},
			AuthUsers:  authUsers,
		}))
}

func TestUserHandler_DeleteAccount_Unauthenticated(t *testing.T) {
	h := newDeletingUserHandler(mock.NewStore(), &recordingAuthUsers{})

	resp, err := h.DeleteAccount(context.Background(), connect.NewRequest(&appv1.DeleteAccountRequest{}))

	assert.Nil(t, resp)
	assertConnectCode(t, err, connect.CodeUnauthenticated)
}

func TestUserHandler_DeleteAccount_UnconfiguredIsUnimplemented(t *testing.T) {
	h := NewUserHandler(application.NewUserService(application.UserServiceDeps{}))
	ctx := auth.ContextWithPrincipal(context.Background(), auth.Principal{Kind: auth.PrincipalKindUser, SubjectID: "owner"})

	_, err := h.DeleteAccount(ctx, connect.NewRequest(&appv1.DeleteAccountRequest{}))

	assertConnectCode(t, err, connect.CodeUnimplemented)
}

// The account deleted is always the caller's own, taken from the ID token.
func TestUserHandler_DeleteAccount_DeletesTheCallersOwnAccount(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	// The mock names workspaces "ws-<name>", so the two need different names.
	ownerAccount, err := store.GetOrCreateAccount(ctx, "owner")
	require.NoError(t, err)
	ownerWorkspace, err := store.CreateWorkspace(ctx, ownerAccount.AccountID, "owners")
	require.NoError(t, err)
	otherAccount, err := store.GetOrCreateAccount(ctx, "other")
	require.NoError(t, err)
	otherWorkspace, err := store.CreateWorkspace(ctx, otherAccount.AccountID, "others")
	require.NoError(t, err)
	require.NoError(t, store.UpsertWorkspaceMember(ctx, otherWorkspace.WorkspaceID, "owner", domain.WorkspaceRoleEditor, "other"))
	authUsers := &recordingAuthUsers{}
	h := newDeletingUserHandler(store, authUsers)
	ownerCtx := auth.ContextWithPrincipal(ctx, auth.Principal{Kind: auth.PrincipalKindUser, SubjectID: "owner"})

	_, err = h.DeleteAccount(ownerCtx, connect.NewRequest(&appv1.DeleteAccountRequest{}))

	require.NoError(t, err)
	assert.Equal(t, []string{"owner"}, authUsers.deleted)
	_, err = store.GetAccountByUser(ctx, "owner")
	assert.ErrorIs(t, err, domain.ErrNotFound)
	_, err = store.GetWorkspace(ctx, ownerWorkspace.WorkspaceID)
	assert.ErrorIs(t, err, domain.ErrNotFound, "the caller's own workspace goes")
	role, err := store.GetWorkspaceRole(ctx, otherWorkspace.WorkspaceID, "owner")
	require.NoError(t, err)
	assert.Empty(t, role, "the caller leaves the workspaces of others")
	_, err = store.GetWorkspace(ctx, otherWorkspace.WorkspaceID)
	assert.NoError(t, err, "other people's workspaces stay")
}

func TestUserHandler_DeleteAccount_RefusedWhileAPaidPlanRenews(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	mock.CreateUserWorkspaceFixture(t, ctx, store, "owner")
	account, err := store.GetAccountByUser(ctx, "owner")
	require.NoError(t, err)
	account.StripeSubscriptionID = "sub_1"
	account.BillingStatus = string(domain.BillingStatusActive)
	authUsers := &recordingAuthUsers{}
	h := newDeletingUserHandler(store, authUsers)
	ownerCtx := auth.ContextWithPrincipal(ctx, auth.Principal{Kind: auth.PrincipalKindUser, SubjectID: "owner"})

	_, err = h.DeleteAccount(ownerCtx, connect.NewRequest(&appv1.DeleteAccountRequest{}))

	assertConnectCode(t, err, connect.CodeFailedPrecondition)
	assert.Empty(t, authUsers.deleted)
}
