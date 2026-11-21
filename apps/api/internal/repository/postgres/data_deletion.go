package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository"
)

var _ repository.DocumentDeletionRepository = (*Store)(nil)

// A tree node comes from a document when it cites the document as a source or
// when one of the document's processing jobs created, merged into or revised
// it. Sources alone are not enough: a node built from chunks the model did not
// cite has no item_sources row, and only the job's mutation log ties it to the
// document. Nodes made by hand have neither and never match.
const (
	nodeFromDocument = `(
    EXISTS (SELECT 1 FROM item_sources s WHERE s.item_id = tree_items.id AND s.document_id = $2)
    OR EXISTS (
      SELECT 1 FROM job_mutation_logs m
      JOIN document_processing_jobs j ON j.job_id = m.job_id
      WHERE m.target_type = 'item' AND m.target_id = tree_items.id AND j.document_id = $2))`
	nodeFromOtherDocument = `(
    EXISTS (SELECT 1 FROM item_sources s WHERE s.item_id = tree_items.id AND s.document_id <> $2)
    OR EXISTS (
      SELECT 1 FROM job_mutation_logs m
      JOIN document_processing_jobs j ON j.job_id = m.job_id
      WHERE m.target_type = 'item' AND m.target_id = tree_items.id AND j.document_id <> $2))`

	// exclusiveItemsWhere matches the nodes of workspace $1 that come from
	// document $2 and from no other document.
	exclusiveItemsWhere = `
WHERE workspace_id = $1
  AND ` + nodeFromDocument + `
  AND NOT ` + nodeFromOtherDocument
)

func (s *Store) GetDocumentDeletionState(ctx context.Context, documentID string) (*domain.DocumentDeletionState, error) {
	state := &domain.DocumentDeletionState{}
	err := s.db.QueryRowContext(ctx, `
SELECT EXISTS (
  SELECT 1 FROM upload_reservations
  WHERE document_id = $1 AND status = 'reserved' AND expires_at > $2
)
FROM documents
WHERE document_id = $1
`, documentID, nowTime()).Scan(&state.UploadPending)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read document upload state: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT job_id
FROM document_processing_jobs
WHERE document_id = $1
ORDER BY created_at, job_id
`, documentID)
	if err != nil {
		return nil, fmt.Errorf("list document jobs: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var jobID string
		if err := rows.Scan(&jobID); err != nil {
			return nil, fmt.Errorf("scan document job: %w", err)
		}
		state.JobIDs = append(state.JobIDs, jobID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate document jobs: %w", err)
	}
	return state, nil
}

func (s *Store) DeleteDocumentRows(ctx context.Context, documentID string) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var workspaceID string
	err = tx.QueryRowContext(ctx, `
SELECT workspace_id FROM documents WHERE document_id = $1 FOR UPDATE
`, documentID).Scan(&workspaceID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, domain.ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("lock document: %w", err)
	}
	now := nowTime()

	deletedItems, err := deleteNodesOnlyFromDocument(ctx, tx, workspaceID, documentID, now)
	if err != nil {
		return 0, err
	}
	// The nodes left from this document also come from others. Once its
	// sources and jobs cascade away, they may come from only one.
	if _, err := tx.ExecContext(ctx, `
UPDATE tree_items
SET cross_document = (
      SELECT count(DISTINCT d.document_id) FROM (
        SELECT s.document_id FROM item_sources s WHERE s.item_id = tree_items.id
        UNION
        SELECT j.document_id FROM job_mutation_logs m
        JOIN document_processing_jobs j ON j.job_id = m.job_id
        WHERE m.target_type = 'item' AND m.target_id = tree_items.id
      ) AS d
      WHERE d.document_id <> $2
    ) > 1,
    updated_at = $3
WHERE workspace_id = $1
  AND `+nodeFromDocument, workspaceID, documentID, now); err != nil {
		return 0, fmt.Errorf("recount node documents: %w", err)
	}
	if err := releaseConfirmedUploadBytes(ctx, tx, documentID, now); err != nil {
		return 0, err
	}
	// Cascades to document_files, document_chunks, upload_reservations,
	// item_sources and document_processing_jobs (with their capabilities,
	// plans, approvals, checkpoints, mutation logs and job logs).
	if _, err := tx.ExecContext(ctx, `DELETE FROM documents WHERE document_id = $1`, documentID); err != nil {
		return 0, fmt.Errorf("delete document: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return deletedItems, nil
}

// deleteNodesOnlyFromDocument deletes the nodes that come from the document
// and no other. Their kept children are first moved up to the nearest kept
// ancestor; left to the FK's ON DELETE SET NULL they would all jump to the
// workspace root instead.
func deleteNodesOnlyFromDocument(ctx context.Context, tx *sql.Tx, workspaceID, documentID string, now time.Time) (int, error) {
	deleted, err := queryIDSet(ctx, tx, `SELECT id FROM tree_items`+exclusiveItemsWhere, workspaceID, documentID)
	if err != nil {
		return 0, fmt.Errorf("list nodes only the document made: %w", err)
	}
	if len(deleted) == 0 {
		return 0, nil
	}

	parentOf := map[string]string{}
	rows, err := tx.QueryContext(ctx, `
SELECT id, COALESCE(parent_id, '') FROM tree_items WHERE workspace_id = $1
`, workspaceID)
	if err != nil {
		return 0, fmt.Errorf("load workspace tree: %w", err)
	}
	for rows.Next() {
		var id, parentID string
		if err := rows.Scan(&id, &parentID); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan workspace tree: %w", err)
		}
		parentOf[id] = parentID
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate workspace tree: %w", err)
	}

	moves := domain.ReparentAroundDeleted(parentOf, deleted)
	movedIDs := make([]string, 0, len(moves))
	for id := range moves {
		movedIDs = append(movedIDs, id)
	}
	sort.Strings(movedIDs)
	for _, id := range movedIDs {
		newParent := sql.NullString{String: moves[id], Valid: moves[id] != ""}
		if _, err := tx.ExecContext(ctx, `
UPDATE tree_items SET parent_id = $2, updated_at = $3 WHERE id = $1
`, id, newParent, now); err != nil {
			return 0, fmt.Errorf("move node %s up: %w", id, err)
		}
	}

	res, err := tx.ExecContext(ctx, `DELETE FROM tree_items`+exclusiveItemsWhere, workspaceID, documentID)
	if err != nil {
		return 0, fmt.Errorf("delete nodes only the document made: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

// releaseConfirmedUploadBytes gives the document's bytes back to the account's
// storage quota. Only a confirmed upload was ever counted (ConfirmDocumentUpload).
func releaseConfirmedUploadBytes(ctx context.Context, tx *sql.Tx, documentID string, now time.Time) error {
	var accountID string
	var size int64
	err := tx.QueryRowContext(ctx, `
SELECT account_id, actual_size_bytes
FROM upload_reservations
WHERE document_id = $1 AND status = 'confirmed'
`, documentID).Scan(&accountID, &size)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read confirmed upload size: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE accounts
SET storage_used_bytes = GREATEST(storage_used_bytes - $2, 0),
    updated_at = $3
WHERE account_id = $1
`, accountID, size, now); err != nil {
		return fmt.Errorf("release storage usage: %w", err)
	}
	return nil
}

func queryIDSet(ctx context.Context, tx *sql.Tx, query string, args ...any) (map[string]bool, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = true
	}
	return ids, rows.Err()
}
