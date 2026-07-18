package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository/mock"
)

func TestSignInUser_NewUser_CreatesAccountAndGrantsCredit(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	billing := &userTestBilling{}
	svc := NewUserService(UserServiceDeps{
		Users:    store,
		Accounts: store,
		Billing:  billing,
		Logger:   nil,
	})

	now := time.Now().UTC()
	svc.now = func() time.Time { return now }

	userID := "user-1"
	email := "user1@example.com"
	displayName := "User One"

	result, err := svc.SignInUser(ctx, userID, email, displayName)

	require.NoError(t, err)
	assert.True(t, result.IsNewAccount)
	assert.Equal(t, userID, result.User.UserID)
	assert.Equal(t, email, result.User.Email)
	assert.Equal(t, displayName, result.User.DisplayName)
	assert.Equal(t, now.Format(time.RFC3339), result.User.CreatedAt)
	assert.Equal(t, now.Format(time.RFC3339), result.User.LastLoginAt)

	// Check Account creation
	account, err := store.GetAccountByUser(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, userID, account.AccountID)

	// Check Credit grant
	assert.Equal(t, 1, billing.grantFreeCalls)
	assert.Equal(t, userID, billing.lastAccountID)
}

func TestSignInUser_ExistingUser_UpdatesLastLoginAndNoNewCredit(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	billing := &userTestBilling{}
	svc := NewUserService(UserServiceDeps{
		Users:    store,
		Accounts: store,
		Billing:  billing,
		Logger:   nil,
	})

	userID := "user-1"
	email := "user1@example.com"

	// Pre-seed user and account
	initialTime := time.Now().UTC().Add(-1 * time.Hour)
	_, _ = store.UpsertUser(ctx, &domain.User{
		UserID:    userID,
		Email:     email,
		CreatedAt: initialTime.Format(time.RFC3339),
	})
	_, _ = store.CreateAccount(ctx, userID)

	now := time.Now().UTC()
	svc.now = func() time.Time { return now }

	result, err := svc.SignInUser(ctx, userID, email, "Updated Name")

	require.NoError(t, err)
	assert.False(t, result.IsNewAccount)
	assert.Equal(t, userID, result.User.UserID)
	assert.Equal(t, "Updated Name", result.User.DisplayName)
	assert.Equal(t, initialTime.Format(time.RFC3339), result.User.CreatedAt)
	assert.Equal(t, now.Format(time.RFC3339), result.User.LastLoginAt)

	// Check No Credit grant
	assert.Equal(t, 0, billing.grantFreeCalls)
}

// users 行はあるが accounts 行が無い「途中失敗」状態からの自己回復ケース。
// 案A の核心: 毎ログインで呼ぶ前提なので、次回呼び出しで account が作られて
// credit が付与されることを保証する。
func TestSignInUser_OrphanUserRow_RecoversByCreatingAccount(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	billing := &userTestBilling{}
	svc := NewUserService(UserServiceDeps{
		Users:    store,
		Accounts: store,
		Billing:  billing,
		Logger:   nil,
	})

	userID := "user-1"
	email := "user1@example.com"

	// Pre-seed user row only (no account) — simulate a partial-failure recovery.
	initialTime := time.Now().UTC().Add(-1 * time.Hour)
	_, _ = store.UpsertUser(ctx, &domain.User{
		UserID:    userID,
		Email:     email,
		CreatedAt: initialTime.Format(time.RFC3339),
	})

	now := time.Now().UTC()
	svc.now = func() time.Time { return now }

	result, err := svc.SignInUser(ctx, userID, email, "")

	require.NoError(t, err)
	assert.True(t, result.IsNewAccount, "should treat orphan user row as new account")
	assert.Equal(t, initialTime.Format(time.RFC3339), result.User.CreatedAt, "preserves original CreatedAt")
	assert.Equal(t, now.Format(time.RFC3339), result.User.LastLoginAt)

	// Account is created during recovery.
	account, err := store.GetAccountByUser(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, userID, account.AccountID)

	// Credit is granted (billing layer is idempotent anyway).
	assert.Equal(t, 1, billing.grantFreeCalls)
}

type userTestBilling struct {
	BillingUsecase // embedding to satisfy interface
	grantFreeCalls int
	lastAccountID  string
}

func (m *userTestBilling) GrantFreeSignupCredit(ctx context.Context, accountID string) error {
	m.grantFreeCalls++
	m.lastAccountID = accountID
	return nil
}
