package handler

import (
	"context"
	"errors"

	connect "connectrpc.com/connect"
	apiauth "github.com/synthify/backend/apps/api/internal/auth"
	"github.com/synthify/backend/apps/api/internal/repository"
)

// requireUserID は認証済みユーザーの ID を返す。未認証なら Unauthenticated を返す。
// ID 以外を必要としない handler はこちらを使う (大半のケース)。
func requireUserID(ctx context.Context) (string, error) {
	return apiauth.RequireUserID(ctx)
}

// requireUserIDOrShareToken は read RPC 用の認可入口。認証ユーザーなら user ID を、
// 未認証でも公開リンク token が context にあれば空 user ID を返す (service 側が
// token -> workspace の認可を行う)。どちらも無ければ Unauthenticated。
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

// requireUserPrincipal は Principal 全体 (SubjectID + Email) を必要とする handler 用。
// 現状は SyncUser のみが利用する。
func requireUserPrincipal(ctx context.Context) (apiauth.Principal, error) {
	return apiauth.RequireUserPrincipal(ctx)
}

func requireServicePrincipal(ctx context.Context) (apiauth.Principal, error) {
	return apiauth.RequireServicePrincipal(ctx)
}

func requireAdminPrincipal(ctx context.Context) (apiauth.Principal, error) {
	return apiauth.RequireAdminPrincipal(ctx)
}

// authorizeWorkspace / authorizeDocument は現状 job handler だけが残存利用している。
// JobUsecase を新設すれば撤去できる (TODO: docs/improvements/api-layering-cleanup.md 参照)。

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
