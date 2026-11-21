package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/api/internal/domain"
)

// accountDeletionFakes records every step of an account deletion in one log.
type accountDeletionFakes struct {
	events        []string
	plan          domain.AccountDeletionPlan
	activeSince   time.Time
	purgeErr      error
	authErr       error
	rowsDeleted   bool
	authUserGone  bool
	markedDeleted []string
}

func (f *accountDeletionFakes) GetAccountDeletionPlan(_ context.Context, userID string, jobsActiveSince, _ time.Time) (*domain.AccountDeletionPlan, error) {
	f.events = append(f.events, "plan "+userID)
	f.activeSince = jobsActiveSince
	if f.rowsDeleted {
		return &domain.AccountDeletionPlan{}, nil
	}
	plan := f.plan
	return &plan, nil
}

func (f *accountDeletionFakes) MarkWorkspacesDeleted(_ context.Context, ids []string, _ time.Time) error {
	f.events = append(f.events, "mark deleted")
	f.markedDeleted = append(f.markedDeleted, ids...)
	return nil
}

func (f *accountDeletionFakes) PurgeWorkspace(_ context.Context, id string) error {
	if f.purgeErr != nil {
		return f.purgeErr
	}
	f.events = append(f.events, "purge "+id)
	return nil
}

func (f *accountDeletionFakes) DeleteAccountRows(_ context.Context, userID string) error {
	f.events = append(f.events, "rows "+userID)
	f.rowsDeleted = true
	return nil
}

func (f *accountDeletionFakes) DeleteUser(_ context.Context, userID string) error {
	if f.authErr != nil {
		return f.authErr
	}
	f.events = append(f.events, "auth user "+userID)
	f.authUserGone = true
	return nil
}

var accountDeletionNow = time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)

func newTestAccountDeletion(f *accountDeletionFakes) *AccountDeletionService {
	return NewAccountDeletionService(AccountDeletionServiceDeps{
		Repo:       f,
		Workspaces: f,
		AuthUsers:  f,
		Logger:     discardLogger(),
		Now:        func() time.Time { return accountDeletionNow },
	})
}

func TestDeleteAccount_PurgesWorkspacesThenRowsThenSignIn(t *testing.T) {
	f := &accountDeletionFakes{plan: domain.AccountDeletionPlan{
		AccountIDs:   []string{"acc"},
		WorkspaceIDs: []string{"ws1", "ws2"},
	}}

	err := newTestAccountDeletion(f).DeleteAccount(context.Background(), "user")

	require.NoError(t, err)
	assert.Equal(t, []string{
		"plan user",
		"mark deleted",
		"purge ws1",
		"purge ws2",
		"rows user",
		"auth user user",
	}, f.events)
	assert.Equal(t, []string{"ws1", "ws2"}, f.markedDeleted)
	assert.Equal(t, accountDeletionNow.Add(-stuckJobMinAge), f.activeSince, "jobs silent for longer are stuck, not running")
}

func TestDeleteAccount_RefusesWhileAPaidPlanRenews(t *testing.T) {
	f := &accountDeletionFakes{plan: domain.AccountDeletionPlan{WorkspaceIDs: []string{"ws1"}, ActiveSubscription: true}}

	err := newTestAccountDeletion(f).DeleteAccount(context.Background(), "user")

	assert.ErrorIs(t, err, domain.ErrActiveSubscription)
	assert.Equal(t, []string{"plan user"}, f.events, "nothing is deleted")
}

func TestDeleteAccount_RefusesWhileADocumentIsProcessing(t *testing.T) {
	f := &accountDeletionFakes{plan: domain.AccountDeletionPlan{WorkspaceIDs: []string{"ws1"}, Busy: true}}

	err := newTestAccountDeletion(f).DeleteAccount(context.Background(), "user")

	assert.ErrorIs(t, err, domain.ErrDocumentBusy)
	assert.Equal(t, []string{"plan user"}, f.events)
}

// The account rows and the sign-in stay while a workspace cannot be purged, so
// the user can sign in and try again.
func TestDeleteAccount_StopsWhenAWorkspaceCannotBePurged(t *testing.T) {
	f := &accountDeletionFakes{
		plan:     domain.AccountDeletionPlan{WorkspaceIDs: []string{"ws1"}},
		purgeErr: errors.New("storage: permission denied"),
	}

	err := newTestAccountDeletion(f).DeleteAccount(context.Background(), "user")

	require.Error(t, err)
	assert.False(t, f.rowsDeleted)
	assert.False(t, f.authUserGone)
}

// If only the sign-in user could not be deleted, calling again finishes it.
func TestDeleteAccount_RetryFinishesAfterSignInFailure(t *testing.T) {
	f := &accountDeletionFakes{
		plan:    domain.AccountDeletionPlan{WorkspaceIDs: []string{"ws1"}},
		authErr: errors.New("firebase unavailable"),
	}
	svc := newTestAccountDeletion(f)

	require.Error(t, svc.DeleteAccount(context.Background(), "user"))
	assert.True(t, f.rowsDeleted)

	f.authErr = nil
	require.NoError(t, svc.DeleteAccount(context.Background(), "user"))
	assert.True(t, f.authUserGone)
}

func TestDeleteAccount_RequiresAUser(t *testing.T) {
	f := &accountDeletionFakes{}

	err := newTestAccountDeletion(f).DeleteAccount(context.Background(), "")

	assert.ErrorIs(t, err, domain.ErrInvalidArgument)
	assert.Empty(t, f.events)
}
