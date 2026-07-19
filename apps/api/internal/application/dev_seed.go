package application

import (
	"context"
	"fmt"

	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository"
)

const devSeedWorkspaceName = "Synthify Dev Seed"

type DevSeedUsecase interface {
	SeedWorkspace(ctx context.Context, userID string) (*DevSeedResult, error)
}

type DevSeedService struct {
	accounts   repository.AccountRepository
	workspaces repository.WorkspaceRepository
	tree       repository.TreeRepository
	items      repository.ItemRepository
}

type DevSeedServiceDeps struct {
	Accounts   repository.AccountRepository
	Workspaces repository.WorkspaceRepository
	Tree       repository.TreeRepository
	Items      repository.ItemRepository
}

type DevSeedResult struct {
	Workspace        *domain.Workspace `json:"workspace"`
	CreatedWorkspace bool              `json:"created_workspace"`
	CreatedItemCount int               `json:"created_item_count"`
	TotalItemCount   int               `json:"total_item_count"`
}

func NewDevSeedService(deps DevSeedServiceDeps) *DevSeedService {
	return &DevSeedService{
		accounts:   deps.Accounts,
		workspaces: deps.Workspaces,
		tree:       deps.Tree,
		items:      deps.Items,
	}
}

func (s *DevSeedService) SeedWorkspace(ctx context.Context, userID string) (*DevSeedResult, error) {
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	account, err := s.accounts.GetOrCreateAccount(ctx, userID)
	if err != nil {
		return nil, err
	}

	workspace, createdWorkspace, err := s.findOrCreateSeedWorkspace(ctx, userID, account.AccountID)
	if err != nil {
		return nil, err
	}

	createdItems, totalItems, err := s.seedTree(ctx, workspace.WorkspaceID, userID)
	if err != nil {
		return nil, err
	}

	return &DevSeedResult{
		Workspace:        workspace,
		CreatedWorkspace: createdWorkspace,
		CreatedItemCount: createdItems,
		TotalItemCount:   totalItems,
	}, nil
}

func (s *DevSeedService) findOrCreateSeedWorkspace(ctx context.Context, userID, accountID string) (*domain.Workspace, bool, error) {
	workspaces, err := s.workspaces.ListWorkspacesByUser(ctx, userID)
	if err != nil {
		return nil, false, err
	}
	for _, workspace := range workspaces {
		if workspace.Name == devSeedWorkspaceName {
			return workspace, false, nil
		}
	}
	workspace, err := s.workspaces.CreateWorkspace(ctx, accountID, devSeedWorkspaceName)
	if err != nil {
		return nil, false, err
	}
	return workspace, true, nil
}

type devSeedNode struct {
	key         string
	parentKey   string
	title       string
	description string
}

var devSeedNodes = []devSeedNode{
	{
		key:         "overview",
		parentKey:   "root",
		title:       "Product Overview",
		description: "How Synthify turns documents into an explorable knowledge tree.",
	},
	{
		key:         "documents",
		parentKey:   "root",
		title:       "Document Pipeline",
		description: "Upload, extraction, chunking, and source tracking for workspace documents.",
	},
	{
		key:         "worker",
		parentKey:   "root",
		title:       "AI Worker",
		description: "Asynchronous jobs that analyze documents and persist structured items.",
	},
	{
		key:         "tree",
		parentKey:   "root",
		title:       "Knowledge Tree",
		description: "Workspace root, document roots, generated nodes, links, and evidence.",
	},
	{
		key:         "ops",
		parentKey:   "root",
		title:       "Operations",
		description: "Sharing, billing, job status, and observability surfaces.",
	},
	{
		key:         "chunks",
		parentKey:   "documents",
		title:       "Source Chunks",
		description: "Small source-backed text units used as evidence for generated nodes.",
	},
	{
		key:         "lifecycle",
		parentKey:   "worker",
		title:       "Job Lifecycle",
		description: "Queued, running, waiting for approval, succeeded, and failed states.",
	},
	{
		key:         "roles",
		parentKey:   "ops",
		title:       "Workspace Roles",
		description: "Owner, editor, and viewer permissions for collaborative workspaces.",
	},
}

func (s *DevSeedService) seedTree(ctx context.Context, workspaceID, userID string) (int, int, error) {
	existingItems, err := s.tree.GetTreeByWorkspace(ctx, workspaceID)
	if err != nil {
		return 0, 0, err
	}

	itemByParentTitle := make(map[string]*domain.Item, len(existingItems))
	for _, item := range existingItems {
		itemByParentTitle[parentTitleKey(item.ParentID, item.Title)] = item
	}

	// node 直属モデル: 専用 root item は無く、"root" を親に持つ node は
	// workspace 直下 (parent_id NULL = parentID "") に作る。
	idsByKey := map[string]string{"root": ""}
	created := 0
	for _, spec := range devSeedNodes {
		parentID, ok := idsByKey[spec.parentKey]
		if !ok {
			return created, len(itemByParentTitle), fmt.Errorf("missing parent %q for seed node %q", spec.parentKey, spec.key)
		}
		if item := itemByParentTitle[parentTitleKey(parentID, spec.title)]; item != nil {
			idsByKey[spec.key] = item.ItemID
			continue
		}
		item, err := s.items.CreateItem(ctx, workspaceID, spec.title, spec.description, parentID, userID)
		if err != nil {
			return created, len(itemByParentTitle), err
		}
		idsByKey[spec.key] = item.ItemID
		itemByParentTitle[parentTitleKey(parentID, spec.title)] = item
		created++
	}
	return created, len(itemByParentTitle), nil
}

func parentTitleKey(parentID, title string) string {
	return parentID + "\x00" + title
}
