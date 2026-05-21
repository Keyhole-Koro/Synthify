package service

import (
	"context"
	"errors"
	"time"

	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository"
	"github.com/synthify/backend/internal/platform/applog"
)

type UserService struct {
	users    repository.UserRepository
	accounts repository.AccountRepository
	billing  BillingUsecase
	logger   applog.Logger
	now      func() time.Time
}

func NewUserService(users repository.UserRepository, accounts repository.AccountRepository, billing BillingUsecase, logger applog.Logger) *UserService {
	if logger == nil {
		logger = applog.NoopLogger{}
	}
	return &UserService{
		users:    users,
		accounts: accounts,
		billing:  billing,
		logger:   logger,
		now:      time.Now,
	}
}

type SyncUserResult struct {
	User      *domain.User
	IsNewUser bool
}

func (s *UserService) SyncUser(ctx context.Context, userID, email, displayName string) (*SyncUserResult, error) {
	if userID == "" {
		return nil, errors.New("user_id is required")
	}

	// 1. Check if user exists in our users table
	existingUser, err := s.users.GetUser(ctx, userID)
	isNewUser := false

	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			isNewUser = true
		} else {
			return nil, err
		}
	}

	// 2. Upsert user info (updates last_login_at)
	user := &domain.User{
		UserID:      userID,
		Email:       email,
		DisplayName: displayName,
		LastLoginAt: s.now().UTC().Format(time.RFC3339),
	}
	if isNewUser {
		user.CreatedAt = user.LastLoginAt
	} else {
		user.CreatedAt = existingUser.CreatedAt
	}

	updatedUser, err := s.users.UpsertUser(ctx, user)
	if err != nil {
		return nil, err
	}

	// 3. Ensure Account exists (if new user, create it)
	var account *domain.Account
	if isNewUser {
		account, err = s.accounts.CreateAccount(ctx, userID)
	} else {
		account, err = s.accounts.GetAccountByUser(ctx, userID)
	}
	if err != nil {
		return nil, err
	}

	// 4. If it's a new user, grant free signup credit
	if isNewUser && s.billing != nil {
		if err := s.billing.GrantFreeSignupCredit(ctx, account.AccountID); err != nil {
			s.logger.Error(ctx, "user.sync_user.grant_credit_failed", err, map[string]any{
				"user_id":    userID,
				"account_id": account.AccountID,
			})
			// We don't fail the whole sync just because of credit grant failure,
			// but it's not ideal.
		}
	}

	return &SyncUserResult{
		User:      updatedUser,
		IsNewUser: isNewUser,
	}, nil
}
