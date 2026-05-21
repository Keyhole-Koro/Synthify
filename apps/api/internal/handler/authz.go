package handler

import (
	"context"
	"errors"

	connect "connectrpc.com/connect"
	"github.com/synthify/backend/apps/api/internal/middleware"
	"github.com/synthify/backend/apps/api/internal/repository"
)

// requireUserID は認証済みユーザーの ID を返す。未認証なら Unauthenticated を返す。
// ID 以外を必要としない handler はこちらを使う (大半のケース)。
func requireUserID(ctx context.Context) (string, error) {
	user, ok := middleware.CurrentUser(ctx)
	if !ok || user.ID == "" {
		return "", connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}
	return user.ID, nil
}

// requireAuthUser は AuthUser 全体 (ID + Email) を必要とする handler 用。
// 現状は SyncUser のみが利用する。
func requireAuthUser(ctx context.Context) (middleware.AuthUser, error) {
	user, ok := middleware.CurrentUser(ctx)
	if !ok || user.ID == "" {
		return middleware.AuthUser{}, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}
	return user, nil
}

// authorizeWorkspace / authorizeDocument は現状 job handler だけが残存利用している。
// JobUsecase を新設すれば撤去できる (TODO: docs/improvements/api-layering-cleanup.md 参照)。

func authorizeWorkspace(ctx context.Context, repo repository.WorkspaceRepository, workspaceID string) error {
	userID, err := requireUserID(ctx)
	if err != nil {
		return err
	}
	if !repo.IsWorkspaceAccessible(ctx, workspaceID, userID) {
		return connect.NewError(connect.CodePermissionDenied, errors.New("workspace access denied"))
	}
	return nil
}

func authorizeDocument(
	ctx context.Context,
	workspaceRepo repository.WorkspaceRepository,
	documentRepo repository.DocumentRepository,
	documentID string,
	expectedWorkspaceID string,
) error {
	// 認証を先にチェック。未認証ユーザーに「document が存在するか」を NotFound 経由で
	// 漏らさないため。
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
