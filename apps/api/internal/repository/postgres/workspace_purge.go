package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository"
)

var _ repository.WorkspacePurgeRepository = (*Store)(nil)

func (s *Store) ListWorkspacesDeletedBefore(ctx context.Context, cutoff time.Time, limit int) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT workspace_id
FROM workspaces
WHERE deleted_at IS NOT NULL AND deleted_at <= $1
ORDER BY deleted_at, workspace_id
LIMIT $2
`, cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("list deleted workspaces: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan deleted workspace: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) GetWorkspaceStorageOwners(ctx context.Context, workspaceID string) (*domain.WorkspaceStorageOwners, error) {
	var deleted bool
	err := s.db.QueryRowContext(ctx, `
SELECT true FROM workspaces WHERE workspace_id = $1 AND deleted_at IS NOT NULL
`, workspaceID).Scan(&deleted)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("check deleted workspace: %w", err)
	}
	owners := &domain.WorkspaceStorageOwners{}
	if owners.DocumentIDs, err = s.queryIDs(ctx, `
SELECT document_id FROM documents WHERE workspace_id = $1 ORDER BY document_id
`, workspaceID); err != nil {
		return nil, fmt.Errorf("list workspace documents: %w", err)
	}
	if owners.JobIDs, err = s.queryIDs(ctx, `
SELECT job_id FROM document_processing_jobs WHERE workspace_id = $1 ORDER BY job_id
`, workspaceID); err != nil {
		return nil, fmt.Errorf("list workspace jobs: %w", err)
	}
	return owners, nil
}

func (s *Store) PurgeWorkspaceRows(ctx context.Context, workspaceID string) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var accountID string
	err = tx.QueryRowContext(ctx, `
SELECT account_id FROM workspaces WHERE workspace_id = $1 AND deleted_at IS NOT NULL FOR UPDATE
`, workspaceID).Scan(&accountID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("lock deleted workspace: %w", err)
	}

	type release struct {
		accountID string
		bytes     int64
	}
	var releases []release
	rows, err := tx.QueryContext(ctx, `
SELECT account_id, sum(actual_size_bytes)::INT8
FROM upload_reservations
WHERE workspace_id = $1 AND status = 'confirmed'
GROUP BY account_id
ORDER BY account_id
`, workspaceID)
	if err != nil {
		return false, fmt.Errorf("sum confirmed uploads: %w", err)
	}
	for rows.Next() {
		var r release
		if err := rows.Scan(&r.accountID, &r.bytes); err != nil {
			rows.Close()
			return false, fmt.Errorf("scan confirmed uploads: %w", err)
		}
		releases = append(releases, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("iterate confirmed uploads: %w", err)
	}
	now := nowTime()
	for _, r := range releases {
		if _, err := tx.ExecContext(ctx, `
UPDATE accounts
SET storage_used_bytes = GREATEST(storage_used_bytes - $2, 0),
    updated_at = $3
WHERE account_id = $1
`, r.accountID, r.bytes, now); err != nil {
			return false, fmt.Errorf("release storage usage: %w", err)
		}
	}

	// Cascades to documents (and everything under them), tree_items,
	// item_aliases, workspace_members, workspace_share_links, upload
	// reservations, processing jobs with their logs, and dynamic_tools.
	if _, err := tx.ExecContext(ctx, `DELETE FROM workspaces WHERE workspace_id = $1`, workspaceID); err != nil {
		return false, fmt.Errorf("delete workspace: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) queryIDs(ctx context.Context, query string, args ...any) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
