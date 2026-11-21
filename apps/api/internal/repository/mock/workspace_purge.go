package mock

import (
	"context"
	"time"

	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository"
)

var _ repository.WorkspacePurgeRepository = (*Store)(nil)

// The mock's DeleteWorkspace removes the workspace at once instead of marking
// it deleted, so there is never a deleted workspace left to purge.

func (s *Store) ListWorkspacesDeletedBefore(context.Context, time.Time, int) ([]string, error) {
	return nil, nil
}

func (s *Store) GetWorkspaceStorageOwners(context.Context, string) (*domain.WorkspaceStorageOwners, error) {
	return nil, domain.ErrNotFound
}

func (s *Store) PurgeWorkspaceRows(context.Context, string) (bool, error) {
	return false, nil
}
