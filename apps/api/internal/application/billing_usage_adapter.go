package application

import (
	"context"
	"errors"

	"github.com/synthify/backend/apps/api/internal/repository"
	"github.com/synthify/backend/internal/domain"
	platformusage "github.com/synthify/backend/internal/platform/billing/usage"
)

// usageRepoAdapter bridges the api-side UsageRepository and AccountRepository
// onto the platform/billing/usage Repository interface. Both sides use the
// internal/domain types; the adapter only maps not-found errors and narrows
// the account to a snapshot. No business logic lives here.
type usageRepoAdapter struct {
	usage    repository.UsageRepository
	accounts repository.AccountRepository
}

// newUsageRepoAdapter creates a new adapter implementing platformusage.Repository.
func newUsageRepoAdapter(usage repository.UsageRepository, accounts repository.AccountRepository) *usageRepoAdapter {
	return &usageRepoAdapter{usage: usage, accounts: accounts}
}

// GetModelPricing retrieves pricing configuration for the specified model.
func (a *usageRepoAdapter) GetModelPricing(ctx context.Context, model string) (*domain.ModelPricing, error) {
	p, err := a.usage.GetModelPricing(ctx, model)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, platformusage.ErrNotFound
		}
		return nil, err
	}
	if p == nil {
		return nil, platformusage.ErrNotFound
	}
	return p, nil
}

// GetCreditBalance returns the active credit balance in minor units for the account.
func (a *usageRepoAdapter) GetCreditBalance(ctx context.Context, accountID string) (int64, error) {
	return a.usage.GetCreditBalance(ctx, accountID)
}

// GrantCredit persists a newly granted or consumed credit for an account.
func (a *usageRepoAdapter) GrantCredit(ctx context.Context, grant *domain.CreditGrant) error {
	return a.usage.GrantCredit(ctx, grant)
}

// GetAccount fetches an account snapshot for usage enforcement purposes.
func (a *usageRepoAdapter) GetAccount(ctx context.Context, accountID string) (*platformusage.AccountSnapshot, error) {
	account, err := a.accounts.GetAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, domain.ErrNotFound
	}
	return &platformusage.AccountSnapshot{
		AccountID: account.AccountID,
		Plan:      account.Plan,
	}, nil
}

// RecordAccounting logs a usage event into the usage repository and checks budget limits.
// The repository may fill in EventID and CreatedAt on ev.
func (a *usageRepoAdapter) RecordAccounting(ctx context.Context, ev *domain.UsageEvent, date string) (bool, error) {
	_, exceeded, err := a.usage.RecordUsageAccounting(ctx, ev, date)
	return exceeded, err
}
