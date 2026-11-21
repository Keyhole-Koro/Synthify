package application

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository"
)

// WorkspacePurgePort purges one workspace marked deleted (WorkspacePurger).
type WorkspacePurgePort interface {
	PurgeWorkspace(ctx context.Context, workspaceID string) error
}

// AuthUserDeleter deletes a user from the sign-in provider. A user that is
// already gone counts as deleted.
type AuthUserDeleter interface {
	DeleteUser(ctx context.Context, userID string) error
}

// AccountDeletionService deletes the caller's account (退会).
type AccountDeletionService struct {
	repo       repository.AccountDeletionRepository
	workspaces WorkspacePurgePort
	authUsers  AuthUserDeleter
	logger     *slog.Logger
	now        func() time.Time
}

type AccountDeletionServiceDeps struct {
	Repo       repository.AccountDeletionRepository
	Workspaces WorkspacePurgePort
	AuthUsers  AuthUserDeleter
	Logger     *slog.Logger
	Now        func() time.Time
}

func NewAccountDeletionService(deps AccountDeletionServiceDeps) *AccountDeletionService {
	s := &AccountDeletionService{
		repo:       deps.Repo,
		workspaces: deps.Workspaces,
		authUsers:  deps.AuthUsers,
		logger:     deps.Logger,
		now:        deps.Now,
	}
	if s.logger == nil {
		s.logger = slog.New(slog.DiscardHandler)
	}
	if s.now == nil {
		s.now = time.Now
	}
	return s
}

// DeleteAccount deletes the user's account for good, in this order:
//
//  1. the workspaces of the accounts the user alone belongs to are marked
//     deleted (they disappear for everyone at once) and purged right away,
//     stored files first, without waiting for the retention period;
//  2. the account rows: memberships in other people's workspaces, the user's
//     ID on what they added there, the accounts with their usage and billing
//     records, and the users row;
//  3. the Firebase Authentication user, last, so a failure before it leaves
//     the user able to sign in and try again.
//
// Every step can run again, so a call that failed part way is finished by
// the next one. Documents the user uploaded to other people's workspaces stay
// there: the tree they fed belongs to that workspace.
//
// It is refused while one of those accounts has a paid plan that renews
// (ErrActiveSubscription) and while a document in one of those workspaces is
// uploading or being processed (ErrDocumentBusy).
func (s *AccountDeletionService) DeleteAccount(ctx context.Context, userID string) error {
	if userID == "" {
		return domain.ErrInvalidArgument
	}
	now := s.now().UTC()
	plan, err := s.repo.GetAccountDeletionPlan(ctx, userID, now.Add(-stuckJobMinAge), now)
	if err != nil {
		return err
	}
	if plan.ActiveSubscription {
		return domain.ErrActiveSubscription
	}
	if plan.Busy {
		return domain.ErrDocumentBusy
	}
	if err := s.repo.MarkWorkspacesDeleted(ctx, plan.WorkspaceIDs, now); err != nil {
		return err
	}
	for _, workspaceID := range plan.WorkspaceIDs {
		if err := s.workspaces.PurgeWorkspace(ctx, workspaceID); err != nil {
			return fmt.Errorf("purge workspace %s: %w", workspaceID, err)
		}
	}
	if err := s.repo.DeleteAccountRows(ctx, userID); err != nil {
		return err
	}
	if err := s.authUsers.DeleteUser(ctx, userID); err != nil {
		return fmt.Errorf("delete sign-in user: %w", err)
	}
	s.logger.Info("account.deleted",
		"user_id", userID,
		"accounts", len(plan.AccountIDs),
		"workspaces", len(plan.WorkspaceIDs),
	)
	return nil
}
