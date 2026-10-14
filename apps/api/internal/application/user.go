package application

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository"
)

// UserUsecase defines the use-case interface for user sign-in and provisioning.
type UserUsecase interface {
	SignInUser(ctx context.Context, userID, email, displayName string) (*SignInUserResult, error)
}

// UserService coordinates user profile persistence, account creation, and signup credit assignment.
type UserService struct {
	users    repository.UserRepository
	accounts repository.AccountRepository
	billing  UserBillingUsecase
	logger   *slog.Logger
	now      func() time.Time
}

// UserBillingUsecase specifies the billing actions needed during user provisioning.
type UserBillingUsecase interface {
	GrantFreeSignupCredit(ctx context.Context, accountID string) error
}

// UserServiceDeps defines dependencies required to instantiate a UserService.
type UserServiceDeps struct {
	Users    repository.UserRepository
	Accounts repository.AccountRepository
	Billing  UserBillingUsecase
	Logger   *slog.Logger
}

// NewUserService creates a new UserService instance.
func NewUserService(deps UserServiceDeps) *UserService {
	return &UserService{
		users:    deps.Users,
		accounts: deps.Accounts,
		billing:  deps.Billing,
		logger:   deps.Logger,
		now:      time.Now,
	}
}

// SignInUserResult contains the provisioned user entity and flags indicating whether an account was newly created.
type SignInUserResult struct {
	User *domain.User
	// IsNewAccount indicates whether a new account was created during this sign-in call.
	// Also true if recovering from an incomplete sign-up where the user existed without an account.
	IsNewAccount bool
}

// SignInUser provisions or updates an authenticated user in the database.
// It upserts the user row, ensures an associated account exists, and grants initial signup credits.
// The operation is idempotent and automatically recovers from previous partial failures.
func (s *UserService) SignInUser(ctx context.Context, userID, email, displayName string) (*SignInUserResult, error) {
	if userID == "" {
		return nil, errors.New("user_id is required")
	}

	user, err := s.upsertUserRow(ctx, userID, email, displayName)
	if err != nil {
		return nil, err
	}

	account, isNewAccount, err := s.ensureAccount(ctx, userID)
	if err != nil {
		return nil, err
	}

	user.AccountID = account.AccountID

	if isNewAccount {
		s.grantSignupCreditIfFirstTime(ctx, userID, account.AccountID)
	}

	return &SignInUserResult{User: user, IsNewAccount: isNewAccount}, nil
}

// upsertUserRow inserts or updates the user profile, preserving the original CreatedAt timestamp if present.
func (s *UserService) upsertUserRow(ctx context.Context, userID, email, displayName string) (*domain.User, error) {
	existing, err := s.users.GetUser(ctx, userID)
	isNew := false
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
		isNew = true
	}

	now := s.now().UTC().Format(time.RFC3339)
	user := &domain.User{
		UserID:      userID,
		Email:       email,
		DisplayName: displayName,
		LastLoginAt: now,
	}
	if isNew {
		user.CreatedAt = now
	} else {
		user.CreatedAt = existing.CreatedAt
	}

	return s.users.UpsertUser(ctx, user)
}

// ensureAccount retrieves an existing account for the user or provisions a new one if missing.
func (s *UserService) ensureAccount(ctx context.Context, userID string) (*domain.Account, bool, error) {
	existing, err := s.accounts.GetAccountByUser(ctx, userID)
	if err == nil {
		return existing, false, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, false, err
	}
	created, err := s.accounts.CreateAccount(ctx, userID)
	if err != nil {
		return nil, false, err
	}
	return created, true, nil
}

// grantSignupCreditIfFirstTime attempts to allocate initial free credits to the newly created account.
func (s *UserService) grantSignupCreditIfFirstTime(ctx context.Context, userID, accountID string) {
	if s.billing == nil {
		return
	}
	if err := s.billing.GrantFreeSignupCredit(ctx, accountID); err != nil {
		s.logger.Error("user.sign_in.grant_credit_failed",
			"error", err.Error(),
			"user_id", userID,
			"account_id", accountID,
		)
	}
}
