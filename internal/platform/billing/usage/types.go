// Package usage owns the usage-billing pipeline: pricing lookup, credit
// drawdown, accounting persistence, and (in PR 8b) the Stripe meter
// outbox. Both api and worker call into it so the same accounting rules
// apply regardless of which service produced the event.
//
// Domain types are intentionally redefined here rather than imported from
// apps/api/internal/domain so that the platform package has no dependency
// on either service. The api and worker domains can later be collapsed
// onto these types in a separate PR.
package usage

import "errors"

var (
	ErrEventInvalid = errors.New("usage event is missing required fields")
)

// PaidVia indicates how a usage event was funded.
type PaidVia string

const (
	PaidViaCredit PaidVia = "credit"
	PaidViaStripe PaidVia = "stripe"
	PaidViaMixed  PaidVia = "mixed"
)

// CreditType discriminates the source of an account credit grant.
type CreditType string

const (
	CreditTypeFree     CreditType = "free"
	CreditTypePurchase CreditType = "purchase"
	CreditTypeAdmin    CreditType = "admin"
	CreditTypeConsumed CreditType = "consumed"
)

// Event is a single LLM usage record produced by the worker and recorded
// against an account's usage / credit / Stripe-meter ledger.
type Event struct {
	EventID           string
	AccountID         string
	WorkspaceID       string
	JobID             string
	Model             string
	InputTokens       int64
	OutputTokens      int64
	CostMinor         int64
	Currency          string
	PaidVia           PaidVia
	CreditAmountMinor int64
	StripeAmountMinor int64
	CreatedAt         string
}

// ModelPricing is the per-million-token rate for a given model. cost_minor
// for one event is computed as
// (input_tokens * input_rate + output_tokens * output_rate) / 1_000_000.
type ModelPricing struct {
	Model                    string
	InputCostPerMTokenMinor  int64
	OutputCostPerMTokenMinor int64
	Currency                 string
	DisplayMultiplier        float64
}

// CreditGrant is an entry in the account_credits ledger. AmountMinor is
// negative for consumption.
type CreditGrant struct {
	CreditID    string
	AccountID   string
	CreditType  CreditType
	AmountMinor int64
	Currency    string
	Note        string
	GrantedBy   string
	GrantedAt   string
}

// RecordResult summarises how a Record call was settled.
type RecordResult struct {
	EventID           string
	Cost              string
	BudgetExceeded    bool
	CreditStopped     bool
	PaidVia           PaidVia
	CreditAmountMinor int64
	StripeAmountMinor int64
}
