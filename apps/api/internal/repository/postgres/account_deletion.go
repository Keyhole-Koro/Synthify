package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository"
)

var _ repository.AccountDeletionRepository = (*Store)(nil)

// soleAccountsOfUser selects the accounts user $1 is the only member of.
const soleAccountsOfUser = `
SELECT au.account_id FROM account_users au
WHERE au.user_id = $1
  AND NOT EXISTS (SELECT 1 FROM account_users o WHERE o.account_id = au.account_id AND o.user_id <> $1)`

func (s *Store) GetAccountDeletionPlan(ctx context.Context, userID string, jobsActiveSince, now time.Time) (*domain.AccountDeletionPlan, error) {
	plan := &domain.AccountDeletionPlan{}
	var err error
	if plan.AccountIDs, err = s.queryIDs(ctx, soleAccountsOfUser+` ORDER BY au.account_id`, userID); err != nil {
		return nil, fmt.Errorf("list the user's accounts: %w", err)
	}
	if plan.WorkspaceIDs, err = s.queryIDs(ctx, `
SELECT workspace_id FROM workspaces WHERE account_id IN (`+soleAccountsOfUser+`)
ORDER BY workspace_id
`, userID); err != nil {
		return nil, fmt.Errorf("list the user's workspaces: %w", err)
	}
	if err := s.db.QueryRowContext(ctx, `
SELECT
  EXISTS (
    SELECT 1 FROM accounts a
    WHERE a.account_id IN (`+soleAccountsOfUser+`)
      AND a.stripe_subscription_id <> ''
      AND a.billing_status NOT IN ('free', 'canceled')
      AND NOT a.cancel_at_period_end
  ),
  EXISTS (
    SELECT 1 FROM document_processing_jobs j
    JOIN workspaces w ON w.workspace_id = j.workspace_id
    WHERE w.account_id IN (`+soleAccountsOfUser+`)
      AND j.status IN ('queued', 'running', 'retryable')
      AND j.updated_at > $2
  ) OR EXISTS (
    SELECT 1 FROM upload_reservations r
    JOIN workspaces w ON w.workspace_id = r.workspace_id
    WHERE w.account_id IN (`+soleAccountsOfUser+`)
      AND r.status = 'reserved'
      AND r.expires_at > $3
  )
`, userID, jobsActiveSince, now).Scan(&plan.ActiveSubscription, &plan.Busy); err != nil {
		return nil, fmt.Errorf("check the user's plan and jobs: %w", err)
	}
	return plan, nil
}

func (s *Store) MarkWorkspacesDeleted(ctx context.Context, workspaceIDs []string, now time.Time) error {
	for _, id := range workspaceIDs {
		if _, err := s.db.ExecContext(ctx, `
UPDATE workspaces SET deleted_at = $2, updated_at = $2
WHERE workspace_id = $1 AND deleted_at IS NULL
`, id, now); err != nil {
			return fmt.Errorf("mark workspace %s deleted: %w", id, err)
		}
	}
	return nil
}

func (s *Store) DeleteAccountRows(ctx context.Context, userID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var workspacesLeft bool
	if err := tx.QueryRowContext(ctx, `
SELECT EXISTS (SELECT 1 FROM workspaces WHERE account_id IN (`+soleAccountsOfUser+`))
`, userID).Scan(&workspacesLeft); err != nil {
		return fmt.Errorf("check the user's workspaces: %w", err)
	}
	if workspacesLeft {
		return fmt.Errorf("%w: the user's accounts still have workspaces", domain.ErrConflict)
	}

	for _, statement := range []struct{ what, query string }{
		{"leave other workspaces", `DELETE FROM workspace_members WHERE user_id = $1`},
		{"clear invitations sent", `UPDATE workspace_members SET invited_by = '' WHERE invited_by = $1`},
		{"clear uploaded_by", `UPDATE documents SET uploaded_by = '' WHERE uploaded_by = $1`},
		{"clear requested_by", `UPDATE document_processing_jobs SET requested_by = '' WHERE requested_by = $1`},
		{"clear created_by", `UPDATE tree_items SET created_by = '' WHERE created_by = $1`},
		// Cascades to account_users, usage_events, account_usage_daily,
		// account_credits, invoices, payment_methods and upload_reservations.
		{"delete the user's accounts", `DELETE FROM accounts WHERE account_id IN (` + soleAccountsOfUser + `)`},
		{"leave shared accounts", `DELETE FROM account_users WHERE user_id = $1`},
		{"delete the user", `DELETE FROM users WHERE user_id = $1`},
	} {
		if _, err := tx.ExecContext(ctx, statement.query, userID); err != nil {
			return fmt.Errorf("%s: %w", statement.what, err)
		}
	}
	return tx.Commit()
}
