package application

import (
	"context"
	"log/slog"

	"github.com/synthify/backend/apps/api/internal/repository"
	"github.com/synthify/backend/internal/domain"
)

// ItemService manages knowledge tree item lifecycle, evidence retrieval, and alias approvals.
type ItemService struct {
	repo       repository.ItemRepository
	workspaces repository.WorkspaceRepository
	logger     *slog.Logger
}

// ItemServiceDeps holds dependencies required to construct an ItemService.
type ItemServiceDeps struct {
	Repo       repository.ItemRepository
	Workspaces repository.WorkspaceRepository
	Logger     *slog.Logger
}

// NewItemService creates a new ItemService instance.
func NewItemService(deps ItemServiceDeps) *ItemService {
	return &ItemService{
		repo:       deps.Repo,
		workspaces: deps.Workspaces,
		logger:     deps.Logger,
	}
}

// authorizeWorkspace ensures the user has read access to the workspace.
func (s *ItemService) authorizeWorkspace(ctx context.Context, workspaceID, userID string) error {
	ok, err := s.workspaces.IsWorkspaceAccessible(ctx, workspaceID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}
	return nil
}

// authorizeWrite verifies that the user holds editor or owner role in the workspace.
func (s *ItemService) authorizeWrite(ctx context.Context, workspaceID, userID string) error {
	role, err := s.workspaces.GetWorkspaceRole(ctx, workspaceID, userID)
	if err != nil {
		return err
	}
	if !role.CanWrite() {
		return domain.ErrForbidden
	}
	return nil
}

// GetItem retrieves a single knowledge tree item, verifying workspace accessibility and item ownership.
func (s *ItemService) GetItem(ctx context.Context, itemID, workspaceID, userID string) (*domain.Item, error) {
	if err := s.authorizeWorkspace(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	item, err := s.repo.GetItem(ctx, itemID)
	if err != nil {
		return nil, err
	}
	if item.WorkspaceID != workspaceID {
		return nil, domain.ErrForbidden
	}
	return item, nil
}

// GetItemEvidence retrieves source chunks and citation references backing a specific knowledge item.
func (s *ItemService) GetItemEvidence(ctx context.Context, itemID, workspaceID, userID string) (*domain.ItemEvidence, error) {
	item, err := s.GetItem(ctx, itemID, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	sources, err := s.repo.ListItemSources(ctx, item.ItemID)
	if err != nil {
		return nil, err
	}
	sourceRefs, err := s.repo.ListItemSourceRefs(ctx, item.ItemID)
	if err != nil {
		return nil, err
	}
	return &domain.ItemEvidence{
		Sources:    sources,
		SourceRefs: sourceRefs,
	}, nil
}

// CreateItem creates a new node in the knowledge tree under the given parent item.
func (s *ItemService) CreateItem(ctx context.Context, workspaceID, label, description, parentID, userID string) (*domain.Item, error) {
	if err := s.authorizeWrite(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	// The parent must be in the same workspace. Children are listed by
	// parent_id alone, so a foreign parent would put this item into another
	// workspace's tree.
	if parentID != "" {
		parent, err := s.repo.GetItem(ctx, parentID)
		if err != nil {
			return nil, err
		}
		if parent.WorkspaceID != workspaceID {
			return nil, domain.ErrForbidden
		}
	}
	return s.repo.CreateItem(ctx, workspaceID, label, description, parentID, userID)
}

// ApproveAlias approves an alias relationship between canonical and alias items.
func (s *ItemService) ApproveAlias(ctx context.Context, workspaceID, canonicalItemID, aliasItemID, userID string) error {
	if err := s.authorizeWrite(ctx, workspaceID, userID); err != nil {
		return err
	}
	if err := s.ensureAliasPair(ctx, workspaceID, canonicalItemID, aliasItemID); err != nil {
		return err
	}
	return s.repo.ApproveAlias(ctx, workspaceID, canonicalItemID, aliasItemID)
}

// RejectAlias rejects a proposed alias relationship between canonical and alias items.
func (s *ItemService) RejectAlias(ctx context.Context, workspaceID, canonicalItemID, aliasItemID, userID string) error {
	if err := s.authorizeWrite(ctx, workspaceID, userID); err != nil {
		return err
	}
	if err := s.ensureAliasPair(ctx, workspaceID, canonicalItemID, aliasItemID); err != nil {
		return err
	}
	return s.repo.RejectAlias(ctx, workspaceID, canonicalItemID, aliasItemID)
}

// ensureAliasPair checks that an alias decision links two distinct items of
// workspaceID. Write access to one workspace must not let a caller record
// aliases that point at another workspace's items.
func (s *ItemService) ensureAliasPair(ctx context.Context, workspaceID, canonicalItemID, aliasItemID string) error {
	if canonicalItemID == aliasItemID {
		return domain.ErrInvalidArgument
	}
	for _, itemID := range []string{canonicalItemID, aliasItemID} {
		item, err := s.repo.GetItem(ctx, itemID)
		if err != nil {
			return err
		}
		if item.WorkspaceID != workspaceID {
			return domain.ErrForbidden
		}
	}
	return nil
}
