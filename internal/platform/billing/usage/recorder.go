package usage

import (
	"context"
	"errors"
	"log/slog"
)

// ErrNotFound is returned by Repository.GetModelPricing when no pricing row
// matches the model. Recorder treats this as "cost 0, persist anyway for
// forensics" rather than a hard error.
var ErrNotFound = errors.New("usage: not found")

// AccountSnapshot is the subset of account data Recorder needs to choose
// between credit and Stripe drawdown. Plan equals "free" stops the meter
// when credit is exhausted.
type AccountSnapshot struct {
	AccountID string
	Plan      string
}

// Repository is the persistence contract Recorder depends on. Both the api
// and worker Postgres stores already implement these methods (api side
// historically called UsageRepository); worker will start implementing the
// same surface in PR 9.
type Repository interface {
	GetModelPricing(ctx context.Context, model string) (*ModelPricing, error)
	GetCreditBalance(ctx context.Context, accountID string) (int64, error)
	GrantCredit(ctx context.Context, grant *CreditGrant) error
	GetAccount(ctx context.Context, accountID string) (*AccountSnapshot, error)
	// RecordAccounting persists the raw event, upserts daily rollup, and
	// updates the account's running period totals in a single transaction.
	// The bool return indicates whether the account budget has just been
	// exceeded by this event.
	RecordAccounting(ctx context.Context, ev *Event, date string) (budgetExceeded bool, err error)
}

// Clock lets tests freeze time. nil falls back to a default time.Now-based
// implementation supplied via NewRecorder's defaults.
type Clock interface {
	NowRFC3339Date() string
}

// Recorder turns a raw Event into pricing + credit/stripe split + Postgres
// persistence. It does not talk to Stripe directly; the Stripe meter event
// is delivered by a separate flusher (see PR 8b).
type Recorder struct {
	repo   Repository
	clock  Clock
	logger *slog.Logger
}

func NewRecorder(repo Repository, clock Clock, logger *slog.Logger) *Recorder {
	return &Recorder{repo: repo, clock: clock, logger: logger}
}

// Record applies pricing, draws down credit, persists the event, and
// returns a settlement summary. ev is mutated in place to record the chosen
// PaidVia, CostMinor, Currency, CreditAmountMinor, and StripeAmountMinor —
// callers that need the original snapshot should copy beforehand.
func (r *Recorder) Record(ctx context.Context, ev *Event) (*RecordResult, error) {
	if ev == nil || ev.EventID == "" || ev.AccountID == "" || ev.Model == "" {
		return nil, ErrEventInvalid
	}

	// 1. Pricing lookup. Unknown model -> cost 0 + warn but persist for forensics.
	currency := "usd"
	costMinor := int64(0)
	pricing, err := r.repo.GetModelPricing(ctx, ev.Model)
	switch {
	case err == nil && pricing != nil:
		costMinor = ComputeCostMinor(pricing, ev.InputTokens, ev.OutputTokens)
		if pricing.Currency != "" {
			currency = pricing.Currency
		}
	case errors.Is(err, ErrNotFound):
		r.logger.Warn("billing.record_usage.no_pricing",
			"model", ev.Model,
			"account_id", ev.AccountID,
		)
	default:
		r.logger.Error("billing.record_usage.pricing_lookup_failed", "error", err.Error(), "model", ev.Model)
	}

	ev.CostMinor = costMinor
	ev.Currency = currency

	// 2. Credit drawdown. balance >= cost  -> all credit; partial -> mixed;
	//    none + usage_based -> stripe; none + free -> stopped.
	creditStopped := false
	creditPortion := int64(0)
	stripePortion := int64(0)
	paidVia := PaidViaStripe

	if costMinor > 0 {
		balance, balErr := r.repo.GetCreditBalance(ctx, ev.AccountID)
		if balErr != nil {
			r.logger.Warn("billing.record_usage.balance_lookup_failed", "error", balErr.Error(), "account_id", ev.AccountID)
		}
		account, accErr := r.repo.GetAccount(ctx, ev.AccountID)
		isFree := accErr == nil && account != nil && account.Plan == "free"

		switch {
		case balance >= costMinor:
			creditPortion = costMinor
			paidVia = PaidViaCredit
		case balance > 0:
			creditPortion = balance
			stripePortion = costMinor - balance
			paidVia = PaidViaMixed
			if isFree {
				creditStopped = true
			}
		case isFree:
			creditStopped = true
			paidVia = PaidViaCredit
		default:
			stripePortion = costMinor
			paidVia = PaidViaStripe
		}

		if creditPortion > 0 {
			deduct := &CreditGrant{
				CreditID:    "deduct-" + ev.EventID,
				AccountID:   ev.AccountID,
				CreditType:  CreditTypeConsumed,
				AmountMinor: -creditPortion,
				Currency:    currency,
				Note:        "usage:" + ev.EventID,
				GrantedBy:   "system",
				GrantedAt:   r.clock.NowRFC3339Date(),
			}
			if err := r.repo.GrantCredit(ctx, deduct); err != nil {
				r.logger.Warn("billing.record_usage.credit_deduct_failed",
					"error", err.Error(),
					"event_id", ev.EventID,
					"account_id", ev.AccountID,
				)
			}
		}
		if creditStopped {
			r.logger.Info("billing.record_usage.credit_exhausted",
				"account_id", ev.AccountID,
				"event_id", ev.EventID,
				"balance", balance,
				"cost", costMinor,
			)
		}
	}

	ev.PaidVia = paidVia
	ev.CreditAmountMinor = creditPortion
	ev.StripeAmountMinor = stripePortion

	// 3. Persist raw event, daily rollup, and account accumulator atomically.
	date := r.clock.NowRFC3339Date()[:10] // YYYY-MM-DD
	exceeded, err := r.repo.RecordAccounting(ctx, ev, date)
	if err != nil {
		r.logger.Error("billing.record_usage.accounting_failed", "error", err.Error(), "event_id", ev.EventID, "account_id", ev.AccountID)
		return nil, err
	}

	return &RecordResult{
		EventID:           ev.EventID,
		Cost:              FormatMinor(costMinor, currency),
		BudgetExceeded:    exceeded || creditStopped,
		CreditStopped:     creditStopped,
		PaidVia:           paidVia,
		CreditAmountMinor: creditPortion,
		StripeAmountMinor: stripePortion,
	}, nil
}

// ProratedTokens splits (inputTokens, outputTokens) by the ratio
// stripePortion/totalCost, rounding the input side up so that small mixed
// events still report at least 1 input token to the Stripe meter.
func ProratedTokens(inputTokens, outputTokens, totalCost, stripePortion int64) (int64, int64) {
	if totalCost <= 0 || stripePortion >= totalCost {
		return inputTokens, outputTokens
	}
	stripeIn := (inputTokens*stripePortion + totalCost - 1) / totalCost
	stripeOut := (outputTokens * stripePortion) / totalCost
	if stripeIn > inputTokens {
		stripeIn = inputTokens
	}
	if stripeOut > outputTokens {
		stripeOut = outputTokens
	}
	return stripeIn, stripeOut
}
