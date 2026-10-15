package handler

import (
	"context"
	"errors"

	connect "connectrpc.com/connect"
	apiauth "github.com/synthify/backend/apps/api/internal/auth"
	"github.com/synthify/backend/apps/api/internal/repository"
)

// requireUserID extracts the authenticated user ID from context, returning connect.CodeUnauthenticated if missing.
func requireUserID(ctx context.Context) (string, error) {
	return apiauth.RequireUserID(ctx)
}

// requireUserIDOrShareToken extracts a user ID or verifies that a public share token is present in context.
// Returns an empty userID if a share token is present, or returns connect.CodeUnauthenticated if neither exists.
func requireUserIDOrShareToken(ctx context.Context) (string, error) {
	userID, err := requireUserID(ctx)
	if err == nil {
		return userID, nil
	}
	if _, ok := apiauth.ShareTokenFromContext(ctx); ok {
		return "", nil
	}
	return "", err
}

// requireUserPrincipal validates and extracts the full authenticated user principal.
func requireUserPrincipal(ctx context.Context) (apiauth.Principal, error) {
	return apiauth.RequireUserPrincipal(ctx)
}

// requireServicePrincipal validates that the request carries a valid backend service principal token.
func requireServicePrincipal(ctx context.Context) (apiauth.Principal, error) {
	return apiauth.RequireServicePrincipal(ctx)
}

// requireAdminPrincipal validates that the request carries an administrative principal token.
func requireAdminPrincipal(ctx context.Context) (apiauth.Principal, error) {
	return apiauth.RequireAdminPrincipal(ctx)
}

// authorizeWorkspace verifies workspace accessibility directly against the workspace repository.
func authorizeWorkspace(ctx context.Context, repo repository.WorkspaceRepository, workspaceID string) error {
	userID, err := requireUserID(ctx)
	if err != nil {
		return err
	}
	ok, err := repo.IsWorkspaceAccessible(ctx, workspaceID, userID)
	if err != nil {
		return toError(err)
	}
	if !ok {
		return connect.NewError(connect.CodePermissionDenied, errors.New("workspace access denied"))
	}
	return nil
}

// authorizeDocument verifies document presence and workspace ownership for the calling user.
func authorizeDocument(
	ctx context.Context,
	workspaceRepo repository.WorkspaceRepository,
	documentRepo repository.DocumentRepository,
	documentID string,
	expectedWorkspaceID string,
) error {
	if _, err := requireUserID(ctx); err != nil {
		return err
	}
	doc, err := documentRepo.GetDocument(ctx, documentID)
	if err != nil {
		return toError(err)
	}
	if expectedWorkspaceID != "" && doc.WorkspaceID != expectedWorkspaceID {
		return connect.NewError(connect.CodePermissionDenied, errors.New("document does not belong to workspace"))
	}
	return authorizeWorkspace(ctx, workspaceRepo, doc.WorkspaceID)
}
