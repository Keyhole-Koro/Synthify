package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository"
	sharedstorage "github.com/synthify/backend/internal/platform/storage"
)

// defaultWorkspacePurgeBatch caps how many workspaces one sweep purges; the
// rest wait for the next sweep, so one run stays short.
const defaultWorkspacePurgeBatch = 10

// WorkspacePurger deletes for good the workspaces that were deleted at least a
// retention period ago. DeleteWorkspace only marks a workspace deleted
// (workspaces.deleted_at): it disappears for its users, but every row and
// stored file stays until the purge.
type WorkspacePurger struct {
	repo      repository.WorkspacePurgeRepository
	objects   repository.ObjectPrefixDeleter
	retention time.Duration
	batchSize int
	logger    *slog.Logger
	now       func() time.Time
}

type WorkspacePurgerDeps struct {
	Repo    repository.WorkspacePurgeRepository
	Objects repository.ObjectPrefixDeleter
	// Retention is how long a deleted workspace is kept before it is purged.
	Retention time.Duration
	// BatchSize caps how many workspaces one sweep purges (default 10).
	BatchSize int
	Logger    *slog.Logger
	Now       func() time.Time
}

func NewWorkspacePurger(deps WorkspacePurgerDeps) *WorkspacePurger {
	p := &WorkspacePurger{
		repo:      deps.Repo,
		objects:   deps.Objects,
		retention: deps.Retention,
		batchSize: deps.BatchSize,
		logger:    deps.Logger,
		now:       deps.Now,
	}
	if p.batchSize <= 0 {
		p.batchSize = defaultWorkspacePurgeBatch
	}
	if p.logger == nil {
		p.logger = slog.New(slog.DiscardHandler)
	}
	if p.now == nil {
		p.now = time.Now
	}
	return p
}

// PurgeDeletedWorkspaces purges up to BatchSize workspaces deleted at least
// Retention ago, oldest first. A workspace that fails stays marked deleted and
// is tried again by the next sweep; the others are still purged. It returns
// how many were purged and the first error.
func (p *WorkspacePurger) PurgeDeletedWorkspaces(ctx context.Context) (int, error) {
	cutoff := p.now().UTC().Add(-p.retention)
	ids, err := p.repo.ListWorkspacesDeletedBefore(ctx, cutoff, p.batchSize)
	if err != nil {
		return 0, fmt.Errorf("list deleted workspaces: %w", err)
	}
	purged := 0
	var firstErr error
	for _, id := range ids {
		if err := p.PurgeWorkspace(ctx, id); err != nil {
			p.logger.Error("workspace.purge_failed", "workspace_id", id, "error", err.Error())
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		purged++
	}
	return purged, firstErr
}

// PurgeWorkspace deletes a workspace that is marked deleted: first its stored
// files, then its rows. If the files cannot be deleted the rows stay, so the
// next attempt still knows which files to delete. A workspace that is not
// marked deleted, or is already gone, is left alone.
func (p *WorkspacePurger) PurgeWorkspace(ctx context.Context, workspaceID string) error {
	owners, err := p.repo.GetWorkspaceStorageOwners(ctx, workspaceID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, prefix := range workspaceStoragePrefixes(workspaceID, owners) {
		if _, err := p.objects.DeleteObjectsWithPrefix(ctx, prefix); err != nil {
			return fmt.Errorf("delete stored files of workspace %s: %w", workspaceID, err)
		}
	}
	purged, err := p.repo.PurgeWorkspaceRows(ctx, workspaceID)
	if err != nil {
		return err
	}
	if purged {
		p.logger.Info("workspace.purged",
			"workspace_id", workspaceID,
			"documents", len(owners.DocumentIDs),
			"jobs", len(owners.JobIDs),
		)
	}
	return nil
}

// workspaceStoragePrefixes lists where a workspace's data is stored: the
// worker's per-document caches and per-job checkpoints, then the workspace's
// own prefix holding every document's original and extracted files.
func workspaceStoragePrefixes(workspaceID string, owners *domain.WorkspaceStorageOwners) []string {
	prefixes := make([]string, 0, len(owners.DocumentIDs)+len(owners.JobIDs)+1)
	for _, documentID := range owners.DocumentIDs {
		prefixes = append(prefixes, sharedstorage.DocumentCachePrefix(documentID))
	}
	for _, jobID := range owners.JobIDs {
		prefixes = append(prefixes, sharedstorage.JobCheckpointPrefix(jobID))
	}
	return append(prefixes, sharedstorage.WorkspaceObjectPrefix(workspaceID))
}
