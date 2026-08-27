package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository"
	"github.com/synthify/backend/apps/api/internal/stif"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

// TreeImportService appends externally authored knowledge to a workspace tree.
// Phase 1 understands one format, STIF; see
// docs/improvements/stif-tree-item-import-format.md.
type TreeImportService struct {
	items      repository.ItemRepository
	workspaces repository.WorkspaceRepository
	transactor repository.Transactor
	logger     *slog.Logger
}

type TreeImportServiceDeps struct {
	Items      repository.ItemRepository
	Workspaces repository.WorkspaceRepository
	Transactor repository.Transactor
	Logger     *slog.Logger
}

func NewTreeImportService(deps TreeImportServiceDeps) *TreeImportService {
	return &TreeImportService{
		items:      deps.Items,
		workspaces: deps.Workspaces,
		transactor: deps.Transactor,
		logger:     deps.Logger,
	}
}

// ImportedItem is one item from the import, carrying the id it was given when
// the import was committed (empty on a dry run).
type ImportedItem struct {
	stif.Item
	ItemID string
}

// ImportResult is what both a preview and a committed import return. A parse
// or validation failure is part of the result rather than an error: the import
// UI has to render every diagnostic at once, and a document the user has not
// fixed yet is an ordinary outcome, not a fault.
type ImportResult struct {
	Committed     bool
	Mode          string
	Items         []ImportedItem
	Diagnostics   []stif.Diagnostic
	ImportedCount int
}

// ImportStif parses source and, unless dryRun is set, appends its items to the
// workspace tree in a single transaction. It returns an error only for
// authorization and infrastructure failures; problems with the document itself
// come back as diagnostics with Committed false.
func (s *TreeImportService) ImportStif(ctx context.Context, workspaceID, source, userID string, dryRun bool) (*ImportResult, error) {
	role, err := s.workspaces.GetWorkspaceRole(ctx, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	if !role.CanWrite() {
		return nil, domain.ErrForbidden
	}

	doc, diags := stif.Parse(source)
	// The request's workspace always wins; the document's @tree workspace is a
	// local hint from whatever produced the file, and silently importing into
	// the wrong workspace would be much worse than saying something.
	if doc.WorkspaceID != "" && doc.WorkspaceID != workspaceID {
		diags = append(diags, stif.Diagnostic{
			Severity: stif.SeverityWarning,
			Code:     "workspace_mismatch",
			Message: fmt.Sprintf("the document declares workspace %q but it is being imported into %q; the request wins",
				doc.WorkspaceID, workspaceID),
		})
	}

	result := &ImportResult{Mode: doc.Mode, Diagnostics: diags}
	result.Items = make([]ImportedItem, 0, len(doc.Items))
	for _, item := range doc.Items {
		result.Items = append(result.Items, ImportedItem{Item: item})
	}

	if stif.HasErrors(diags) || dryRun {
		return result, nil
	}
	if len(doc.Items) == 0 {
		return result, nil
	}

	if err := s.commit(ctx, workspaceID, userID, result); err != nil {
		return nil, err
	}
	result.Committed = true
	result.ImportedCount = len(result.Items)

	if s.logger != nil {
		s.logger.InfoContext(ctx, "stif import committed",
			slog.String("workspace_id", workspaceID),
			slog.String("user_id", userID),
			slog.Int("item_count", result.ImportedCount),
		)
	}
	return result, nil
}

// commit writes every item in one transaction, so a document either lands whole
// or not at all: a half-imported tree has dangling parents and is worse than no
// import. Items arrive parents-first, so a child's parent id is always known by
// the time it is written, and siblings arrive in `order` sequence, which is how
// the ordering survives (sibling order is created_at).
func (s *TreeImportService) commit(ctx context.Context, workspaceID, userID string, result *ImportResult) error {
	return s.transactor.WithTx(ctx, func(repos repository.Repositories) error {
		idByLocalPath := make(map[string]string, len(result.Items))

		for i := range result.Items {
			item := &result.Items[i]

			parentID := ""
			if item.ParentLocalPath != "" {
				resolved, ok := idByLocalPath[item.ParentLocalPath]
				if !ok {
					// resolve() guarantees parents come first, so this can only
					// mean the ordering invariant broke.
					return fmt.Errorf("parent %s of %s was not written first", item.ParentLocalPath, item.LocalPath)
				}
				parentID = resolved
			}

			created, err := repos.CreateImportedItem(ctx, &domain.Item{
				WorkspaceID:     workspaceID,
				ParentID:        parentID,
				Title:           item.Title,
				Level:           item.Depth,
				Description:     item.Description,
				Content:         item.Content,
				OverrideCSS:     item.OverrideCSS,
				CreatedBy:       userID,
				GovernanceState: governanceStateFromSTIF(item.State),
				CrossDocument:   item.CrossDocument,
			})
			if err != nil {
				return fmt.Errorf("create item %s: %w", item.LocalPath, err)
			}

			item.ItemID = created.ItemID
			idByLocalPath[item.LocalPath] = created.ItemID

			for _, source := range item.Sources {
				if err := repos.UpsertItemSourceRef(ctx, sourceRefFromSTIF(created.ItemID, source)); err != nil {
					return fmt.Errorf("create source ref for %s: %w", item.LocalPath, err)
				}
			}
		}
		return nil
	})
}

// governanceStateFromSTIF maps a STIF state onto the enum. STIF uses the same
// vocabulary as the column, so this is domain.ItemGovernanceStateFromName plus
// the import default: an imported item is something a person chose to bring in,
// so it is human_curated and the worker must not overwrite it.
func governanceStateFromSTIF(state string) appv1.ItemGovernanceState {
	resolved := domain.ItemGovernanceStateFromName(state)
	if resolved == appv1.ItemGovernanceState_ITEM_GOVERNANCE_STATE_UNSPECIFIED {
		return appv1.ItemGovernanceState_ITEM_GOVERNANCE_STATE_HUMAN_CURATED
	}
	return resolved
}

func sourceRefFromSTIF(itemID string, source stif.Source) *domain.ItemSourceRef {
	return &domain.ItemSourceRef{
		ItemID:     itemID,
		Type:       source.Type,
		URL:        source.URL,
		Repo:       source.Repo,
		Ref:        source.Ref,
		Path:       source.Path,
		LineStart:  source.LineStart,
		LineEnd:    source.LineEnd,
		Kind:       source.Kind,
		ExternalID: source.ExternalID,
		Title:      source.Title,
		Confidence: source.Confidence,
	}
}
