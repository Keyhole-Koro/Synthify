package usage

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
)

type usageRepoStub struct {
	pricing       *ModelPricing
	pricingErr    error
	balance       int64
	balanceErr    error
	account       *AccountSnapshot
	accountErr    error
	grant         *CreditGrant
	accountingErr error
	recorded      *Event
	recordedDate  string
}

func (r *usageRepoStub) GetModelPricing(context.Context, string) (*ModelPricing, error) {
	return r.pricing, r.pricingErr
}
func (r *usageRepoStub) GetCreditBalance(context.Context, string) (int64, error) {
	return r.balance, r.balanceErr
}
func (r *usageRepoStub) GrantCredit(_ context.Context, grant *CreditGrant) error {
	r.grant = grant
	return nil
}
func (r *usageRepoStub) GetAccount(context.Context, string) (*AccountSnapshot, error) {
	return r.account, r.accountErr
}
func (r *usageRepoStub) RecordAccounting(_ context.Context, event *Event, date string) (bool, error) {
	r.recorded = event
	r.recordedDate = date
	return false, r.accountingErr
}

type usageClockStub struct{}

func (usageClockStub) NowRFC3339Date() string { return "2026-01-02T03:04:05Z" }

func newUsageRecorder(repo *usageRepoStub) *Recorder {
	return NewRecorder(repo, usageClockStub{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func usageEvent() *Event {
	return &Event{EventID: "event_1", AccountID: "acct_1", Model: "model_1", InputTokens: 1_000_000}
}

func TestRecorderRejectsIncompleteEvent(t *testing.T) {
	if _, err := newUsageRecorder(&usageRepoStub{}).Record(context.Background(), nil); !errors.Is(err, ErrEventInvalid) {
		t.Fatalf("nil event error = %v", err)
	}
	if _, err := newUsageRecorder(&usageRepoStub{}).Record(context.Background(), &Event{EventID: "event_1"}); !errors.Is(err, ErrEventInvalid) {
		t.Fatalf("incomplete event error = %v", err)
	}
}

func TestRecorderUsesFullCredit(t *testing.T) {
	repo := &usageRepoStub{
		pricing: &ModelPricing{InputCostPerMTokenMinor: 100, Currency: "usd"},
		balance: 100,
		account: &AccountSnapshot{Plan: "usage_based"},
	}
	event := usageEvent()
	result, err := newUsageRecorder(repo).Record(context.Background(), event)
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if result.PaidVia != PaidViaCredit || result.CreditAmountMinor != 100 || result.StripeAmountMinor != 0 {
		t.Fatalf("result = %+v", result)
	}
	if repo.grant == nil || repo.grant.AmountMinor != -100 || repo.recordedDate != "2026-01-02" {
		t.Fatalf("grant/date = %+v/%q", repo.grant, repo.recordedDate)
	}
}

func TestRecorderUsesMixedCreditAndStripe(t *testing.T) {
	repo := &usageRepoStub{
		pricing: &ModelPricing{InputCostPerMTokenMinor: 100, Currency: "usd"},
		balance: 40,
		account: &AccountSnapshot{Plan: "usage_based"},
	}
	result, err := newUsageRecorder(repo).Record(context.Background(), usageEvent())
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if result.PaidVia != PaidViaMixed || result.CreditAmountMinor != 40 || result.StripeAmountMinor != 60 {
		t.Fatalf("result = %+v", result)
	}
}

func TestRecorderStopsFreeAccountWhenCreditIsExhausted(t *testing.T) {
	repo := &usageRepoStub{
		pricing: &ModelPricing{InputCostPerMTokenMinor: 100, Currency: "usd"},
		account: &AccountSnapshot{Plan: "free"},
	}
	result, err := newUsageRecorder(repo).Record(context.Background(), usageEvent())
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if !result.CreditStopped || !result.BudgetExceeded || result.StripeAmountMinor != 0 {
		t.Fatalf("result = %+v", result)
	}
}

func TestRecorderPersistsZeroCostWhenPricingIsMissing(t *testing.T) {
	repo := &usageRepoStub{pricingErr: ErrNotFound}
	result, err := newUsageRecorder(repo).Record(context.Background(), usageEvent())
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if !result.PricingMissing || result.Cost != "0.00" || repo.recorded == nil {
		t.Fatalf("result/recorded = %+v/%+v", result, repo.recorded)
	}
}

func TestRecorderTreatsNilPricingAsMissing(t *testing.T) {
	repo := &usageRepoStub{}
	result, err := newUsageRecorder(repo).Record(context.Background(), usageEvent())
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if !result.PricingMissing || repo.recorded == nil {
		t.Fatalf("result/recorded = %+v/%+v", result, repo.recorded)
	}
}

func TestRecorderReturnsAccountingError(t *testing.T) {
	want := errors.New("accounting unavailable")
	repo := &usageRepoStub{
		pricing:       &ModelPricing{InputCostPerMTokenMinor: 100, Currency: "usd"},
		accountingErr: want,
	}
	if _, err := newUsageRecorder(repo).Record(context.Background(), usageEvent()); !errors.Is(err, want) {
		t.Fatalf("Record() error = %v, want %v", err, want)
	}
}
