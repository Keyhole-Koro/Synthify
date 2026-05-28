package service

import (
	"context"
	"errors"

	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository"
	platformusage "github.com/synthify/backend/internal/platform/billing/usage"
)

// usageRepoAdapter bridges the api-side UsageRepository (which traffics in
// domain.* types) onto the platform/billing/usage Repository interface
// (which has its own type set, kept dependency-free for the worker). It is
// a translation layer only — no business logic lives here.
type usageRepoAdapter struct {
	usage    repository.UsageRepository
	accounts repository.AccountRepository
}

func newUsageRepoAdapter(usage repository.UsageRepository, accounts repository.AccountRepository) *usageRepoAdapter {
	return &usageRepoAdapter{usage: usage, accounts: accounts}
}

func (a *usageRepoAdapter) GetModelPricing(ctx context.Context, model string) (*platformusage.ModelPricing, error) {
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
	return &platformusage.ModelPricing{
		Model:                    p.Model,
		InputCostPerMTokenMinor:  p.InputCostPerMTokenMinor,
		OutputCostPerMTokenMinor: p.OutputCostPerMTokenMinor,
		Currency:                 p.Currency,
		DisplayMultiplier:        p.DisplayMultiplier,
	}, nil
}

func (a *usageRepoAdapter) GetCreditBalance(ctx context.Context, accountID string) (int64, error) {
	return a.usage.GetCreditBalance(ctx, accountID)
}

func (a *usageRepoAdapter) GrantCredit(ctx context.Context, grant *platformusage.CreditGrant) error {
	return a.usage.GrantCredit(ctx, &domain.CreditGrant{
		CreditID:    grant.CreditID,
		AccountID:   grant.AccountID,
		CreditType:  domain.CreditType(grant.CreditType),
		AmountMinor: grant.AmountMinor,
		Currency:    grant.Currency,
		Note:        grant.Note,
		GrantedBy:   grant.GrantedBy,
		GrantedAt:   grant.GrantedAt,
	})
}

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

func (a *usageRepoAdapter) RecordAccounting(ctx context.Context, ev *platformusage.Event, date string) (bool, error) {
	domainEvent := &domain.UsageEvent{
		EventID:           ev.EventID,
		AccountID:         ev.AccountID,
		WorkspaceID:       ev.WorkspaceID,
		JobID:             ev.JobID,
		Model:             ev.Model,
		InputTokens:       ev.InputTokens,
		OutputTokens:      ev.OutputTokens,
		CostMinor:         ev.CostMinor,
		Currency:          ev.Currency,
		PaidVia:           domain.PaidVia(ev.PaidVia),
		CreditAmountMinor: ev.CreditAmountMinor,
		StripeAmountMinor: ev.StripeAmountMinor,
		CreatedAt:         ev.CreatedAt,
	}
	_, exceeded, err := a.usage.RecordUsageAccounting(ctx, domainEvent, date)
	// The repository may set EventID on the domain event; mirror that back.
	ev.EventID = domainEvent.EventID
	ev.CreatedAt = domainEvent.CreatedAt
	return exceeded, err
}
