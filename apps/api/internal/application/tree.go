package application

import (
	"context"
	"errors"
	"log/slog"
	"time"

	apiauth "github.com/synthify/backend/apps/api/internal/auth"
	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository"
)

// defaultSubtreeMaxDepth is the fallback depth limit when traversing a subtree if none is specified.
const defaultSubtreeMaxDepth = 3

// TreeUsecase defines the use-case interface for knowledge tree traversal and pathfinding.
// Each method verifies workspace authorization using either a user ID or a shared access token.
type TreeUsecase interface {
	GetTree(ctx context.Context, workspaceID, userID string) ([]*domain.Item, error)
	GetSubtree(ctx context.Context, workspaceID, itemID, userID string, maxDepth int) ([]*domain.SubtreeItem, error)
	FindPaths(ctx context.Context, workspaceID, sourceItemID, targetItemID, userID string, maxDepth, limit int) ([]*domain.Item, []domain.TreePath, error)
}

// TreeService implements TreeUsecase to query knowledge trees, subtrees, and node paths.
type TreeService struct {
	tree       repository.TreeRepository
	workspaces repository.WorkspaceRepository
	logger     *slog.Logger
}

// TreeServiceDeps defines dependencies required to initialize a TreeService.
type TreeServiceDeps struct {
	Tree       repository.TreeRepository
	Workspaces repository.WorkspaceRepository
	Logger     *slog.Logger
}

// NewTreeService initializes a new TreeService with the given dependencies.
func NewTreeService(deps TreeServiceDeps) *TreeService {
	return &TreeService{
		tree:       deps.Tree,
		workspaces: deps.Workspaces,
		logger:     deps.Logger,
	}
}

// authorizeWorkspace authorizes read access using user membership or share tokens.
func (s *TreeService) authorizeWorkspace(ctx context.Context, workspaceID, userID string) error {
	// Authenticated users are checked against workspace membership.
	if userID != "" {
		ok, err := s.workspaces.IsWorkspaceAccessible(ctx, workspaceID, userID)
		if err != nil {
			return err
		}
		if !ok {
			return domain.ErrForbidden
		}
		return nil
	}
	// Unauthenticated requests check for a valid share token pointing to the workspace.
	return s.authorizeShareToken(ctx, workspaceID)
}

// authorizeShareToken validates that the share token in the context matches the requested workspace.
// Returns ErrForbidden on missing, invalid, or mismatched tokens to avoid leaking workspace existence.
func (s *TreeService) authorizeShareToken(ctx context.Context, workspaceID string) error {
	token, ok := apiauth.ShareTokenFromContext(ctx)
	if !ok {
		return domain.ErrForbidden
	}
	link, err := s.workspaces.ResolveShareLink(ctx, token, time.Now().UTC())
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ErrForbidden
		}
		return err
	}
	if link.WorkspaceID != workspaceID {
		return domain.ErrForbidden
	}
	return nil
}

// GetTree returns all items belonging to the specified workspace after authorization.
func (s *TreeService) GetTree(ctx context.Context, workspaceID, userID string) ([]*domain.Item, error) {
	if workspaceID == "" {
		return nil, errors.New("workspace_id is required")
	}
	if err := s.authorizeWorkspace(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	return s.tree.GetTreeByWorkspace(ctx, workspaceID)
}

// GetSubtree returns the subtree rooted at itemID up to maxDepth levels deep.
// Returns ErrForbidden if the root item belongs to a different workspace,
// or ErrNotFound if no items were found.
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

// FindPaths discovers all directed paths between sourceItemID and targetItemID up to maxDepth.
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
