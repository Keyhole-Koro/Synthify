package service

import (
	"context"
	"errors"
	"log/slog"

	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository"
)

const defaultSubtreeMaxDepth = 3

// TreeUsecase は handler が依存する TreeService の API 表面。
// 各メソッドは userID を受け取り、内部で workspace authz を実施する。
type TreeUsecase interface {
	GetTree(ctx context.Context, workspaceID, userID string) ([]*domain.Item, error)
	GetSubtree(ctx context.Context, workspaceID, itemID, userID string, maxDepth int) ([]*domain.SubtreeItem, error)
	FindPaths(ctx context.Context, workspaceID, sourceItemID, targetItemID, userID string, maxDepth, limit int) ([]*domain.Item, []domain.TreePath, error)
}

type TreeService struct {
	tree       repository.TreeRepository
	workspaces repository.WorkspaceRepository
	logger     *slog.Logger
}

type TreeServiceDeps struct {
	Tree       repository.TreeRepository
	Workspaces repository.WorkspaceRepository
	Logger     *slog.Logger
}

func NewTreeService(deps TreeServiceDeps) *TreeService {
	return &TreeService{
		tree:       deps.Tree,
		workspaces: deps.Workspaces,
		logger:     deps.Logger,
	}
}

func (s *TreeService) authorizeWorkspace(ctx context.Context, workspaceID, userID string) error {
	ok, err := s.workspaces.IsWorkspaceAccessible(ctx, workspaceID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}
	return nil
}

// GetTree は workspace 配下の全 item を返す。
func (s *TreeService) GetTree(ctx context.Context, workspaceID, userID string) ([]*domain.Item, error) {
	if workspaceID == "" {
		return nil, errors.New("workspace_id is required")
	}
	if err := s.authorizeWorkspace(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	return s.tree.GetTreeByWorkspace(ctx, workspaceID)
}

// GetSubtree は itemID をルートとする部分木を返す。
// 取得した部分木が指定 workspace に属さない場合は ErrForbidden を返す
// (item の存在を未認可ユーザーに NotFound 経由で漏らさないため)。
// 部分木が空の場合は ErrNotFound を返す。
func (s *TreeService) GetSubtree(ctx context.Context, workspaceID, itemID, userID string, maxDepth int) ([]*domain.SubtreeItem, error) {
	if workspaceID == "" || itemID == "" {
		return nil, errors.New("workspace_id and item_id are required")
	}
	if err := s.authorizeWorkspace(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	if maxDepth <= 0 {
		maxDepth = defaultSubtreeMaxDepth
	}
	items, err := s.tree.GetSubtree(ctx, itemID, maxDepth)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, domain.ErrNotFound
	}
	if items[0].WorkspaceID != workspaceID {
		return nil, domain.ErrForbidden
	}
	return items, nil
}

// FindPaths は source-target 間の path を検索する。tree は workspace 作成時に
// 必ず一緒に作られている前提なので、ここで GetTree が失敗するのは workspace が
// 存在しない (= 認可前に弾かれているはず) ケースだけ。
func (s *TreeService) FindPaths(ctx context.Context, workspaceID, sourceItemID, targetItemID, userID string, maxDepth, limit int) ([]*domain.Item, []domain.TreePath, error) {
	if workspaceID == "" || sourceItemID == "" || targetItemID == "" {
		return nil, nil, errors.New("workspace_id, source_item_id, target_item_id are required")
	}
	if err := s.authorizeWorkspace(ctx, workspaceID, userID); err != nil {
		return nil, nil, err
	}
	tree, err := s.tree.GetTree(ctx, workspaceID)
	if err != nil {
		return nil, nil, err
	}
	return s.tree.FindPaths(ctx, tree.TreeID, sourceItemID, targetItemID, maxDepth, limit)
}
