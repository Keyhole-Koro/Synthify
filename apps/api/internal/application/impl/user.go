package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository"
)

// UserUsecase は handler が依存する UserService の API 表面。
type UserUsecase interface {
	SignInUser(ctx context.Context, userID, email, displayName string) (*SignInUserResult, error)
}

type UserService struct {
	users    repository.UserRepository
	accounts repository.AccountRepository
	billing  UserBillingUsecase
	logger   *slog.Logger
	now      func() time.Time
}

type UserBillingUsecase interface {
	GrantFreeSignupCredit(ctx context.Context, accountID string) error
}

type UserServiceDeps struct {
	Users    repository.UserRepository
	Accounts repository.AccountRepository
	Billing  UserBillingUsecase
	Logger   *slog.Logger
}

func NewUserService(deps UserServiceDeps) *UserService {
	return &UserService{
		users:    deps.Users,
		accounts: deps.Accounts,
		billing:  deps.Billing,
		logger:   deps.Logger,
		now:      time.Now,
	}
}

type SignInUserResult struct {
	User *domain.User
	// IsNewAccount は今回の呼び出しで account を新規作成したかどうか。
	// 「users 行があるが accounts 行が無い」途中失敗からの自己回復ケースでも true になる。
	IsNewAccount bool
}

// SignInUser は Firebase 認証済みユーザーを我々の DB に provision する。
// クライアントはサインインに成功するたびに呼ぶ:
//  1. users 行を upsert (LastLoginAt を更新、無ければ作成)
//  2. account が無ければ作成し、無料サインアップクレジットを付与
//
// 冪等。何度呼んでも安全。途中失敗 (users 行はあるが accounts が無い等) からも
// 次回呼び出しで自動回復する。
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

// upsertUserRow は users 行を upsert する。毎回呼ばれる前提。
// 既存行があれば CreatedAt を保持し、無ければ now で初期化する。
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

// ensureAccount は account を取得し、無ければ作成する。
// isNewAccount は今回の呼び出しで作成したかどうか。
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

// grantSignupCreditIfFirstTime は無料サインアップクレジットを付与する。
// billing 側で credit_id が "free-signup-<accountID>" 固定で冪等になっているため
// 万一二重で呼ばれても問題ない。失敗時は sync 全体を失敗させずログのみ。
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
