package handler

import (
	"context"
	"errors"

	connect "connectrpc.com/connect"
	"github.com/synthify/backend/apps/api/internal/repository"
)

// authorizeDocumentWrite returns PermissionDenied unless the caller may write
// to the workspace that holds the document: its owner or an editor.
//
// Requesting, approving and rejecting a job's approval decide whether the job
// runs, so they need the role that starting the job needs, not just the read
// access authorizeDocument checks. A viewer only reads the workspace.
func authorizeDocumentWrite(
	ctx context.Context,
	workspaceRepo repository.WorkspaceRepository,
	documentRepo repository.DocumentRepository,
	documentID string,
) error {
	userID, err := requireUserID(ctx)
	if err != nil {
		return err
	}
	doc, err := documentRepo.GetDocument(ctx, documentID)
	if err != nil {
		return toError(err)
	}
	role, err := workspaceRepo.GetWorkspaceRole(ctx, doc.WorkspaceID, userID)
	if err != nil {
		return toError(err)
	}
	if !role.CanWrite() {
		return connect.NewError(connect.CodePermissionDenied, errors.New("workspace write access denied"))
	}
	return nil
}
