package application

import (
	"context"
	"fmt"

	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

// devSeedWorkspaceName is the display name assigned to the development seed workspace.
const devSeedWorkspaceName = "Synthify Dev Seed"

// DevSeedUsecase exposes the interface for seeding a workspace with starter items, documents, and chunks.
type DevSeedUsecase interface {
	SeedWorkspace(ctx context.Context, userID string) (*DevSeedResult, error)
}

// DevSeedService implements DevSeedUsecase to populate test/demo data in local and dev environments.
type DevSeedService struct {
	accounts   repository.AccountRepository
	workspaces repository.WorkspaceRepository
	tree       repository.TreeRepository
	items      repository.ItemRepository
	documents  repository.DocumentRepository
	chunks     repository.DocumentChunkRepository
	jobs       repository.JobRepository
}

// DevSeedServiceDeps defines dependencies required to instantiate DevSeedService.
type DevSeedServiceDeps struct {
	Accounts   repository.AccountRepository
	Workspaces repository.WorkspaceRepository
	Tree       repository.TreeRepository
	Items      repository.ItemRepository
	Documents  repository.DocumentRepository
	Chunks     repository.DocumentChunkRepository
	Jobs       repository.JobRepository
}

// DevSeedResult summarizes the output of the development seeding operation.
type DevSeedResult struct {
	Workspace           *domain.Workspace `json:"workspace"`
	CreatedWorkspace    bool              `json:"created_workspace"`
	CreatedItemCount    int               `json:"created_item_count"`
	TotalItemCount      int               `json:"total_item_count"`
	CreatedDocumentName string            `json:"created_document_name,omitempty"`
}

// NewDevSeedService creates a new DevSeedService with the provided repositories.
func NewDevSeedService(deps DevSeedServiceDeps) *DevSeedService {
	return &DevSeedService{
		accounts:   deps.Accounts,
		workspaces: deps.Workspaces,
		tree:       deps.Tree,
		items:      deps.Items,
		documents:  deps.Documents,
		chunks:     deps.Chunks,
		jobs:       deps.Jobs,
	}
}

// SeedWorkspace sets up or updates the development seed workspace for a user with tree nodes and a processed document.
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

	documentName, err := s.seedDocument(ctx, workspace.WorkspaceID, userID)
	if err != nil {
		return nil, err
	}

	return &DevSeedResult{
		Workspace:           workspace,
		CreatedWorkspace:    createdWorkspace,
		CreatedItemCount:    createdItems,
		TotalItemCount:      totalItems,
		CreatedDocumentName: documentName,
	}, nil
}

// seedDocument creates a mock processed document with chunks and a succeeded processing job,
// enabling citation and retrieval in the workspace chat without needing full background workers.
func (s *DevSeedService) seedDocument(ctx context.Context, workspaceID, userID string) (string, error) {
	if s.documents == nil || s.chunks == nil || s.jobs == nil {
		return "", nil
	}

	existing, err := s.documents.ListDocuments(ctx, workspaceID)
	if err != nil {
		return "", err
	}
	for _, doc := range existing {
		if doc.Filename == devSeedDocumentName {
			return "", nil // already seeded
		}
	}

	doc, _, err := s.documents.CreateDocument(ctx, workspaceID, userID, devSeedDocumentName, "text/markdown", devSeedDocumentSize)
	if err != nil {
		return "", err
	}

	chunks := make([]*domain.DocumentChunk, 0, len(devSeedChunks))
	for i, spec := range devSeedChunks {
		chunks = append(chunks, &domain.DocumentChunk{
			ChunkID:    fmt.Sprintf("%s-chunk-%d", doc.DocumentID, i+1),
			DocumentID: doc.DocumentID,
			Heading:    spec.heading,
			Text:       spec.text,
			SourcePage: i + 1,
		})
	}
	if err := s.chunks.SaveDocumentChunks(ctx, doc.DocumentID, chunks); err != nil {
		return "", err
	}

	job, err := s.jobs.CreateProcessingJob(ctx, doc.DocumentID, workspaceID, userID, appv1.JobType_JOB_TYPE_PROCESS_DOCUMENT)
	if err != nil {
		return "", err
	}
	if err := s.jobs.CompleteProcessingJob(ctx, job.JobID); err != nil {
		return "", err
	}

	return doc.Filename, nil
}

// findOrCreateSeedWorkspace locates an existing "Synthify Dev Seed" workspace or creates a new one.
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

const (
	// devSeedDocumentName is the filename used for the seed document.
	devSeedDocumentName = "synthify-overview.md"
	// devSeedDocumentSize is the mock file size for the seed document.
	devSeedDocumentSize = 4096
)

// devSeedChunk holds heading and text content for a mock document chunk.
type devSeedChunk struct {
	heading string
	text    string
}

// devSeedChunks contains the sample document body used for text extraction and citation retrieval testing.
var devSeedChunks = []devSeedChunk{
	{
		heading: "概要",
		text: "Synthify はアップロードされたドキュメントを解析し、paper-in-paper で読める" +
			"ナレッジツリーに変換するサービスである。利用者は資料を投入するだけで、" +
			"章立てに沿った探索可能な知識構造を得られる。",
	},
	{
		heading: "処理パイプライン",
		text: "アップロードされた資料はまず GCS に保存され、worker が非同期ジョブとして" +
			"テキスト抽出とチャンク分割を行う。各チャンクは埋め込みベクトルとともに保存され、" +
			"ナレッジツリーの各アイテムから出典として参照される。",
	},
	{
		heading: "ナレッジツリー",
		text: "ナレッジツリーの各アイテムは単体で読んで意味が通じる粒度で作られる。" +
			"親アイテムは本文中に子アイテムへのリンクを含み、クリックすると子が親の中に" +
			"展開される。階層の深さに制限は設けていない。",
	},
	{
		heading: "ワークスペースと権限",
		text: "ワークスペースは owner / editor / viewer の3つの role を持つ。" +
			"viewer は閲覧のみ可能で、資料の追加やツリーの変更はできない。" +
			"課金が発生する処理は editor 以上のメンバーだけが実行できる。",
	},
	{
		heading: "結論",
		text: "Synthify の狙いは、読むのに時間がかかる資料を、必要な部分だけ辿れる形に" +
			"変えることである。全文を通読しなくても、知りたい概念から出典まで最短で辿り着ける。",
	},
}

// devSeedNode defines a node specification in the seed tree structure.
type devSeedNode struct {
	key         string
	parentKey   string
	title       string
	description string
}

// devSeedNodes specifies the default hierarchy of seed tree items.
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

// seedTree populates hierarchical items in the seed workspace if they do not yet exist.
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

// parentTitleKey builds a composite key from a parent item ID and node title.
func parentTitleKey(parentID, title string) string {
	return parentID + "\x00" + title
}
