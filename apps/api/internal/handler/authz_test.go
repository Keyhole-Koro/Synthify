package handler

import (
	"context"
	"testing"

	connect "connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/api/internal/auth"
	"github.com/synthify/backend/apps/api/internal/repository/mock"
)

// assertConnectCode fails the test if err is nil or does not carry the expected connect code.
func assertConnectCode(t *testing.T, err error, want connect.Code) {
	t.Helper()
	require.Error(t, err, "expected error with code %v, got nil", want)
	var ce *connect.Error
	require.ErrorAs(t, err, &ce, "expected *connect.Error")
	assert.Equal(t, want, ce.Code(), "connect code")
}

// ── requireUserID ────────────────────────────────────────────────────────────

func TestRequireUserID_NoAuthInContext_ReturnsUnauthenticated(t *testing.T) {
	_, err := requireUserID(context.Background())
	assertConnectCode(t, err, connect.CodeUnauthenticated)
}

func TestRequireUserID_EmptyUserID_ReturnsUnauthenticated(t *testing.T) {
	ctx := auth.ContextWithPrincipal(context.Background(), auth.Principal{Kind: auth.PrincipalKindUser, SubjectID: "", Email: "x@y.com"})
	_, err := requireUserID(ctx)
	assertConnectCode(t, err, connect.CodeUnauthenticated)
}

func TestRequireUserID_ValidUser_ReturnsUserID(t *testing.T) {
	ctx := auth.ContextWithPrincipal(context.Background(), auth.Principal{Kind: auth.PrincipalKindUser, SubjectID: "u1", Email: "u@example.com"})
	userID, err := requireUserID(ctx)
	require.NoError(t, err, "requireUserID")
	assert.Equal(t, "u1", userID, "userID")
}

// ── requireUserPrincipal ─────────────────────────────────────────────────────

func TestRequireUserPrincipal_NoAuthInContext_ReturnsUnauthenticated(t *testing.T) {
	_, err := requireUserPrincipal(context.Background())
	assertConnectCode(t, err, connect.CodeUnauthenticated)
}

func TestRequireUserPrincipal_ValidUser_ReturnsUser(t *testing.T) {
	ctx := auth.ContextWithPrincipal(context.Background(), auth.Principal{Kind: auth.PrincipalKindUser, SubjectID: "u1", Email: "u@example.com"})
	principal, err := requireUserPrincipal(ctx)
	require.NoError(t, err, "requireUserPrincipal")
	assert.Equal(t, "u1", principal.SubjectID, "principal.SubjectID")
	assert.Equal(t, "u@example.com", principal.Email, "principal.Email")
}

func TestRequireUserPrincipal_ServicePrincipal_ReturnsUnauthenticated(t *testing.T) {
	ctx := auth.ContextWithPrincipal(context.Background(), auth.Principal{Kind: auth.PrincipalKindService, SubjectID: auth.InternalServiceSubjectID})
	_, err := requireUserPrincipal(ctx)
	assertConnectCode(t, err, connect.CodeUnauthenticated)
}

func TestRequireServicePrincipal_UserPrincipal_ReturnsPermissionDenied(t *testing.T) {
	ctx := auth.ContextWithPrincipal(context.Background(), auth.Principal{Kind: auth.PrincipalKindUser, SubjectID: "u1", Email: "u@example.com"})
	_, err := requireServicePrincipal(ctx)
	assertConnectCode(t, err, connect.CodePermissionDenied)
}

func TestRequireAdminPrincipal_AdminUser_ReturnsPrincipal(t *testing.T) {
	ctx := auth.ContextWithPrincipal(context.Background(), auth.Principal{Kind: auth.PrincipalKindUser, SubjectID: "admin", Email: "a@example.com", Admin: true})
	principal, err := requireAdminPrincipal(ctx)
	require.NoError(t, err, "requireAdminPrincipal")
	assert.True(t, principal.Admin)
}

func TestRequireAdminPrincipal_NonAdminUser_ReturnsPermissionDenied(t *testing.T) {
	ctx := auth.ContextWithPrincipal(context.Background(), auth.Principal{Kind: auth.PrincipalKindUser, SubjectID: "u1", Email: "u@example.com"})
	_, err := requireAdminPrincipal(ctx)
	assertConnectCode(t, err, connect.CodePermissionDenied)
}

// ── authorizeWorkspace ────────────────────────────────────────────────────────

func TestAuthorizeWorkspace_Unauthenticated_ReturnsUnauthenticated(t *testing.T) {
	store := mock.NewStore()
	err := authorizeWorkspace(context.Background(), store, "any_ws")
	assertConnectCode(t, err, connect.CodeUnauthenticated)
}

func TestAuthorizeWorkspace_NotMember_ReturnsPermissionDenied(t *testing.T) {
	store := mock.NewStore()
	wsID := mock.CreateUserWorkspaceFixture(t, context.Background(), store, "owner").Workspace.WorkspaceID
	ctx := auth.ContextWithPrincipal(context.Background(), auth.Principal{Kind: auth.PrincipalKindUser, SubjectID: "stranger", Email: "s@example.com"})

	err := authorizeWorkspace(ctx, store, wsID)
	assertConnectCode(t, err, connect.CodePermissionDenied)
}

func TestAuthorizeWorkspace_Member_ReturnsNil(t *testing.T) {
	store := mock.NewStore()
	wsID := mock.CreateUserWorkspaceFixture(t, context.Background(), store, "owner").Workspace.WorkspaceID
	ctx := auth.ContextWithPrincipal(context.Background(), auth.Principal{Kind: auth.PrincipalKindUser, SubjectID: "owner", Email: "o@example.com"})

	err := authorizeWorkspace(ctx, store, wsID)
	assert.NoError(t, err, "authorizeWorkspace")
}

// ── authorizeDocument ─────────────────────────────────────────────────────────

func TestAuthorizeDocument_DocumentNotFound_ReturnsNotFound(t *testing.T) {
	store := mock.NewStore()
	ctx := auth.ContextWithPrincipal(context.Background(), auth.Principal{Kind: auth.PrincipalKindUser, SubjectID: "u1", Email: "u@example.com"})

	err := authorizeDocument(ctx, store, store, "nonexistent_doc", "")
	assertConnectCode(t, err, connect.CodeNotFound)
}

func TestAuthorizeDocument_WrongWorkspace_ReturnsPermissionDenied(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	ws1ID := mock.CreateUserWorkspaceFixture(t, ctx, store, "owner").Workspace.WorkspaceID
	ws2ID := mock.CreateUserWorkspaceFixture(t, ctx, store, "owner2").Workspace.WorkspaceID
	doc, _, _ := store.CreateDocument(ctx, ws1ID, "owner", "f.pdf", "application/pdf", 100)
	authedCtx := auth.ContextWithPrincipal(ctx, auth.Principal{Kind: auth.PrincipalKindUser, SubjectID: "owner", Email: "o@example.com"})

	err := authorizeDocument(authedCtx, store, store, doc.DocumentID, ws2ID)
	assertConnectCode(t, err, connect.CodePermissionDenied)
}

func TestAuthorizeDocument_NotMember_ReturnsPermissionDenied(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	wsID := mock.CreateUserWorkspaceFixture(t, ctx, store, "owner").Workspace.WorkspaceID
	doc, _, _ := store.CreateDocument(ctx, wsID, "owner", "f.pdf", "application/pdf", 100)
	authedCtx := auth.ContextWithPrincipal(ctx, auth.Principal{Kind: auth.PrincipalKindUser, SubjectID: "stranger", Email: "s@example.com"})

	err := authorizeDocument(authedCtx, store, store, doc.DocumentID, "")
	assertConnectCode(t, err, connect.CodePermissionDenied)
}

func TestAuthorizeDocument_Member_ReturnsNil(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	wsID := mock.CreateUserWorkspaceFixture(t, ctx, store, "owner").Workspace.WorkspaceID
	doc, _, _ := store.CreateDocument(ctx, wsID, "owner", "f.pdf", "application/pdf", 100)
	authedCtx := auth.ContextWithPrincipal(ctx, auth.Principal{Kind: auth.PrincipalKindUser, SubjectID: "owner", Email: "o@example.com"})

	err := authorizeDocument(authedCtx, store, store, doc.DocumentID, "")
	assert.NoError(t, err, "authorizeDocument")
}

// ── ItemService.GetItem authz (移植先: PR 4 で ItemService に認可を統合) ─────

// ItemService.GetItem の認可テストは service/item_test.go へ。
// item 存在しない / 別 workspace / member 等のシナリオは ItemService の責務。
