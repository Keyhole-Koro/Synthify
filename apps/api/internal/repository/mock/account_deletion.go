package mock

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

var _ repository.AccountDeletionRepository = (*Store)(nil)

// ownedWorkspaceIDsLocked returns the workspaces of the user's account. The
// mock keys accounts by user ID and has one member per account.
func (s *Store) ownedWorkspaceIDsLocked(userID string) []string {
	var ids []string
	for id := range s.workspaces {
		if s.isOwnerLocked(id, userID) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func (s *Store) GetAccountDeletionPlan(ctx context.Context, userID string, jobsActiveSince, now time.Time) (*domain.AccountDeletionPlan, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	plan := &domain.AccountDeletionPlan{}
	account, ok := s.accounts[userID]
	if !ok {
		return plan, nil
	}
	plan.AccountIDs = []string{account.AccountID}
	plan.WorkspaceIDs = s.ownedWorkspaceIDsLocked(userID)
	for id, members := range s.wsMembers {
		if _, ok := members[userID]; ok {
			plan.OtherWorkspaceIDs = append(plan.OtherWorkspaceIDs, id)
		}
	}
	sort.Strings(plan.OtherWorkspaceIDs)
	plan.ActiveSubscription = account.StripeSubscriptionID != "" &&
		account.BillingStatus != string(domain.BillingStatusFree) &&
		account.BillingStatus != string(domain.BillingStatusCanceled) &&
		!account.CancelAtPeriodEnd
	owned := map[string]bool{}
	for _, id := range plan.WorkspaceIDs {
		owned[id] = true
	}
	for _, job := range s.jobs {
		switch job.Status {
		case appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_QUEUED,
			appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_RUNNING,
			appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_RETRYABLE:
		default:
			continue
		}
		updatedAt, err := time.Parse(time.RFC3339, job.UpdatedAt)
		if owned[job.WorkspaceID] && (err != nil || updatedAt.After(jobsActiveSince)) {
			plan.Busy = true
		}
	}
	for _, r := range s.reservations {
		if owned[r.WorkspaceID] && r.Status == "reserved" && r.ExpiresAt.After(now) {
			plan.Busy = true
		}
	}
	return plan, nil
}

// MarkWorkspacesDeleted removes the workspaces at once: the mock has no
// deleted mark, as its DeleteWorkspace shows.
func (s *Store) MarkWorkspacesDeleted(ctx context.Context, workspaceIDs []string, now time.Time) error {
	for _, id := range workspaceIDs {
		if err := s.DeleteWorkspace(ctx, id); err != nil && err != domain.ErrNotFound {
			return err
		}
	}
	return nil
}

func (s *Store) DeleteAccountRows(ctx context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if left := s.ownedWorkspaceIDsLocked(userID); len(left) > 0 {
		return fmt.Errorf("%w: the user's accounts still have workspaces", domain.ErrConflict)
	}
	for _, members := range s.wsMembers {
		delete(members, userID)
		for _, member := range members {
			if member.InvitedBy == userID {
				member.InvitedBy = ""
			}
		}
	}
	for _, doc := range s.documents {
		if doc.UploadedBy == userID {
			doc.UploadedBy = ""
		}
	}
	for _, job := range s.jobs {
		if job.RequestedBy == userID {
			job.RequestedBy = ""
		}
	}
	for _, items := range s.items {
		for _, item := range items {
			if item.CreatedBy == userID {
				item.CreatedBy = ""
			}
		}
	}
	delete(s.accounts, userID)
	delete(s.users, userID)
	return nil
}
