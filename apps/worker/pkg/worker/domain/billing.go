// Package domain defines core domain entities, values, and business errors for the worker service.
package domain

import (
	"errors"
	"fmt"
)

var (
	// ErrBillingProviderNotConfigured indicates that Stripe credentials or configuration are absent.
	ErrBillingProviderNotConfigured = errors.New("billing provider is not configured")
	// ErrBillingProviderMisconfigured indicates that billing provider parameters are invalid.
	ErrBillingProviderMisconfigured = errors.New("billing provider is misconfigured")
	// ErrBillingPlanInvalid indicates that an unrecognized billing plan was specified.
	ErrBillingPlanInvalid = errors.New("billing plan is invalid")
	// ErrBillingCurrencyUnsupported indicates that an unsupported currency was requested.
	ErrBillingCurrencyUnsupported = errors.New("billing currency is unsupported")
	// ErrBillingWebhookSignatureInvalid indicates that a webhook signature failed cryptographic verification.
	ErrBillingWebhookSignatureInvalid = errors.New("billing webhook signature is invalid")

	// Usage-Based Billing (Phase 1-3)

	// ErrBillingUsageEventInvalid indicates that a usage event lacks required identifiers or token counts.
	ErrBillingUsageEventInvalid = errors.New("usage event is missing required fields")
	// ErrBillingBudgetInvalid indicates that the specified monthly budget limit is negative or malformed.
	ErrBillingBudgetInvalid = errors.New("budget limit is invalid")
	// ErrBillingBudgetExceeded indicates that an account has exceeded its configured spending budget.
	ErrBillingBudgetExceeded = errors.New("account budget has been exceeded")
)

// BillingPlan represents the tier or pricing model of an account subscription.
type BillingPlan string

// BillingCurrency represents an ISO 4217 currency code supported by billing.
type BillingCurrency string

// BillingInterval represents the recurring billing cycle interval.
type BillingInterval string

// BillingStatus represents the lifecycle state of a customer's subscription.
type BillingStatus string

const (
	// BillingPlanFree is the default free plan tier.
	BillingPlanFree BillingPlan = "free"
	// BillingPlanUsageBased is the pay-as-you-go metered plan tier.
	BillingPlanUsageBased BillingPlan = "usage_based"
)

const (
	// BillingCurrencyJPY indicates Japanese Yen.
	BillingCurrencyJPY BillingCurrency = "jpy"
	// BillingCurrencyUSD indicates United States Dollar.
	BillingCurrencyUSD BillingCurrency = "usd"
)

const (
	// BillingIntervalMonth represents monthly recurring billing.
	BillingIntervalMonth BillingInterval = "month"
	// BillingIntervalYear represents yearly recurring billing.
	BillingIntervalYear BillingInterval = "year"
)

const (
	// BillingStatusFree represents an account on the free tier.
	BillingStatusFree BillingStatus = "free"
	// BillingStatusCheckoutPending represents an account awaiting checkout completion.
	BillingStatusCheckoutPending BillingStatus = "checkout_pending"
	// BillingStatusActive represents an account with an active paid subscription.
	BillingStatusActive BillingStatus = "active"
	// BillingStatusPastDue represents an account with past due invoice payment.
	BillingStatusPastDue BillingStatus = "past_due"
	// BillingStatusUnpaid represents an unpaid subscription.
	BillingStatusUnpaid BillingStatus = "unpaid"
	// BillingStatusCanceled represents a canceled subscription.
	BillingStatusCanceled BillingStatus = "canceled"
	// BillingStatusIncomplete represents an incomplete checkout attempt.
	BillingStatusIncomplete BillingStatus = "incomplete"
)

// Validate ensures that the billing plan matches a known plan identifier.
func (p BillingPlan) Validate() error {
	switch p {
	case BillingPlanFree, BillingPlanUsageBased:
		return nil
	case "":
		return fmt.Errorf("%w: empty", ErrBillingPlanInvalid)
	default:
		return fmt.Errorf("%w: %s", ErrBillingPlanInvalid, p)
	}
}

// Validate ensures that the billing currency is supported.
func (c BillingCurrency) Validate() error {
	switch c {
	case BillingCurrencyJPY, BillingCurrencyUSD:
		return nil
	case "":
		return fmt.Errorf("%w: empty", ErrBillingCurrencyUnsupported)
	default:
		return fmt.Errorf("%w: %s", ErrBillingCurrencyUnsupported, c)
	}
}

// Validate ensures that the billing interval is either monthly or yearly.
func (i BillingInterval) Validate() error {
	switch i {
	case BillingIntervalMonth, BillingIntervalYear:
		return nil
	case "":
		return nil
	default:
		return fmt.Errorf("billing interval is invalid: %s", i)
	}
}

// BillingCheckoutSession encapsulates the redirect URL for a Stripe Checkout session.
type BillingCheckoutSession struct {
	URL string `json:"url"`
}

// BillingPortalSession encapsulates the URL for the Stripe Customer Portal.
type BillingPortalSession struct {
	URL string `json:"url"`
}

// BillingCustomerRef contains external payment gateway customer identifiers.
type BillingCustomerRef struct {
	ExternalCustomerID string `json:"external_customer_id"`
}

// ProviderWebhookEvent represents a normalized webhook payload received from a payment provider.
type ProviderWebhookEvent struct {
	Provider               string          `json:"provider"`
	EventID                string          `json:"event_id"`
	EventType              string          `json:"event_type"`
	AccountID              string          `json:"account_id,omitempty"`
	ExternalCustomerID     string          `json:"external_customer_id"`
	ExternalSubscriptionID string          `json:"external_subscription_id"`
	Plan                   BillingPlan     `json:"plan"`
	Status                 BillingStatus   `json:"status,omitempty"`
	ExternalPriceID        string          `json:"external_price_id,omitempty"`
	Currency               BillingCurrency `json:"currency,omitempty"`
	AmountMinor            int64           `json:"amount_minor,omitempty"`
	Interval               BillingInterval `json:"interval,omitempty"`
	CurrentPeriodEnd       string          `json:"current_period_end,omitempty"`
	CancelAtPeriodEnd      bool            `json:"cancel_at_period_end,omitempty"`
}

// =========================================================
// Usage-Based Billing (Phase 1-3)
// =========================================================

// PaidVia indicates the payment source used to settle a token usage event.
type PaidVia string

const (
	// PaidViaCredit indicates the cost was settled against available credit balance.
	PaidViaCredit PaidVia = "credit"
	// PaidViaStripe indicates the usage was reported to Stripe's metered billing engine.
	PaidViaStripe PaidVia = "stripe"
	// PaidViaMixed indicates a combination of credit balance and Stripe metered billing.
	PaidViaMixed PaidVia = "mixed"
)

// UsageEvent represents an LLM token consumption record for a job or chat interaction.
type UsageEvent struct {
	EventID           string  `json:"event_id"`
	AccountID         string  `json:"account_id"`
	WorkspaceID       string  `json:"workspace_id,omitempty"`
	JobID             string  `json:"job_id,omitempty"`
	Model             string  `json:"model"`
	InputTokens       int64   `json:"input_tokens"`
	OutputTokens      int64   `json:"output_tokens"`
	CostMinor         int64   `json:"cost_minor"`
	Currency          string  `json:"currency"`
	CreatedAt         string  `json:"created_at"`
	PaidVia           PaidVia `json:"paid_via,omitempty"`
	CreditAmountMinor int64   `json:"credit_amount_minor,omitempty"`
	StripeAmountMinor int64   `json:"stripe_amount_minor,omitempty"`
}

// UsageRecordResult contains the recorded cost and budget status resulting from an ingestion event.
type UsageRecordResult struct {
	EventID           string  `json:"event_id"`
	Cost              string  `json:"cost"`
	BudgetExceeded    bool    `json:"budget_exceeded"`
	CreditStopped     bool    `json:"credit_stopped"` // 無料クレジット残高0で停止
	PaidVia           PaidVia `json:"paid_via,omitempty"`
	CreditAmountMinor int64   `json:"credit_amount_minor,omitempty"`
	StripeAmountMinor int64   `json:"stripe_amount_minor,omitempty"`
}

// ModelUsage aggregates input/output token counts and costs by model.
type ModelUsage struct {
	Model        string `json:"model"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	InputCost    string `json:"input_cost"`
	OutputCost   string `json:"output_cost"`
	TotalCost    string `json:"total_cost"`
	EventCount   int64  `json:"event_count"`
}

// DailyUsage represents the total spend recorded on a specific date.
type DailyUsage struct {
	Date      string `json:"date"`
	TotalCost string `json:"total_cost"`
}

// UsageReport compiles detailed breakdown and aggregate costs across a billing period.
type UsageReport struct {
	AccountID   string       `json:"account_id"`
	PeriodStart string       `json:"period_start"`
	PeriodEnd   string       `json:"period_end"`
	TotalCost   string       `json:"total_cost"`
	Currency    string       `json:"currency"`
	ByModel     []ModelUsage `json:"by_model"`
	ByDay       []DailyUsage `json:"by_day"`
}

// Invoice represents a billing invoice issued by Stripe.
type Invoice struct {
	InvoiceID        string `json:"invoice_id"`
	Amount           string `json:"amount"`
	Currency         string `json:"currency"`
	Status           string `json:"status"`
	HostedInvoiceURL string `json:"hosted_invoice_url"`
	InvoicePDFURL    string `json:"invoice_pdf_url"`
	PeriodStart      string `json:"period_start"`
	PeriodEnd        string `json:"period_end"`
	PaidAt           string `json:"paid_at,omitempty"`
	CreatedAt        string `json:"created_at"`
}

// InvoiceList encapsulates a collection of invoices and upcoming charges.
type InvoiceList struct {
	Invoices          []*Invoice `json:"invoices"`
	UpcomingAmount    string     `json:"upcoming_amount"`
	UpcomingPeriodEnd string     `json:"upcoming_period_end"`
}

// ModelPricing defines per-million-token input and output rates for an LLM model.
type ModelPricing struct {
	Model                    string  `json:"model"`
	InputCostPerMTokenMinor  int64   `json:"input_cost_per_mtoken_minor"`
	OutputCostPerMTokenMinor int64   `json:"output_cost_per_mtoken_minor"`
	Currency                 string  `json:"currency"`
	DisplayMultiplier        float64 `json:"display_multiplier"`
}

// CreditType classifies the origin or state of account credits.
type CreditType string

const (
	// CreditTypeFree represents promotional signup credit.
	CreditTypeFree CreditType = "free"
	// CreditTypePurchased represents purchased credits.
	CreditTypePurchased CreditType = "purchased"
	// CreditTypeConsumed represents consumed credits.
	CreditTypeConsumed CreditType = "consumed"
)

// CreditGrant records an addition of balance to an account.
type CreditGrant struct {
	CreditID    string     `json:"credit_id"`
	AccountID   string     `json:"account_id"`
	CreditType  CreditType `json:"credit_type"`
	AmountMinor int64      `json:"amount_minor"`
	Currency    string     `json:"currency"`
	Note        string     `json:"note"`
	GrantedBy   string     `json:"granted_by"`
	GrantedAt   string     `json:"granted_at"`
}

const (
	// FreeSignupCreditMinor represents the signup promotional credit awarded to new accounts in cents ($1.00).
	FreeSignupCreditMinor int64 = 100
)

// PaymentMethod describes a credit card or payment instrument attached to an account.
type PaymentMethod struct {
	PaymentMethodID string `json:"payment_method_id"`
	Brand           string `json:"brand"`
	Last4           string `json:"last4"`
	ExpMonth        int32  `json:"exp_month"`
	ExpYear         int32  `json:"exp_year"`
	IsDefault       bool   `json:"is_default"`
}
