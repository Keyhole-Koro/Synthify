// Package domain defines the core business entities, aggregates, and value objects for the Synthify platform.
package domain

import (
	"errors"
	"fmt"
)

var (
	// ErrBillingProviderNotConfigured indicates that billing operations were requested but no payment provider is enabled.
	ErrBillingProviderNotConfigured = errors.New("billing provider is not configured")
	// ErrBillingProviderMisconfigured indicates that payment provider credentials or options are invalid.
	ErrBillingProviderMisconfigured = errors.New("billing provider is misconfigured")
	// ErrBillingPlanInvalid indicates that an unrecognized billing plan was requested.
	ErrBillingPlanInvalid = errors.New("billing plan is invalid")
	// ErrBillingCurrencyUnsupported indicates that the requested currency is not supported for billing.
	ErrBillingCurrencyUnsupported = errors.New("billing currency is unsupported")
	// ErrBillingWebhookSignatureInvalid indicates that an incoming payment webhook has an invalid cryptographic signature.
	ErrBillingWebhookSignatureInvalid = errors.New("billing webhook signature is invalid")

	// ErrBillingUsageEventInvalid indicates that a metered usage event lacks required tracking fields.
	ErrBillingUsageEventInvalid = errors.New("usage event is missing required fields")
	// ErrBillingBudgetInvalid indicates that a configured account budget limit is invalid.
	ErrBillingBudgetInvalid = errors.New("budget limit is invalid")
	// ErrBillingBudgetExceeded indicates that an account has reached its configured monthly expenditure threshold.
	ErrBillingBudgetExceeded = errors.New("account budget has been exceeded")
	// ErrBillingUsagePricingMissing is an internal marker error recorded when a model usage event
	// is saved with zero cost because model pricing could not be resolved.
	ErrBillingUsagePricingMissing = errors.New("usage event recorded without pricing")
)

// BillingPlan represents the tier classification of an account subscription.
type BillingPlan string

// BillingCurrency represents an ISO currency code supported for billing.
type BillingCurrency string

// BillingInterval represents the recurring subscription billing period.
type BillingInterval string

// BillingStatus represents the current standing of an account's subscription.
type BillingStatus string

const (
	// BillingPlanFree indicates the default free tier.
	BillingPlanFree BillingPlan = "free"
	// BillingPlanUsageBased indicates pay-as-you-go metered billing.
	BillingPlanUsageBased BillingPlan = "usage_based"
)

const (
	// BillingCurrencyJPY represents Japanese Yen.
	BillingCurrencyJPY BillingCurrency = "jpy"
	// BillingCurrencyUSD represents United States Dollar.
	BillingCurrencyUSD BillingCurrency = "usd"
)

const (
	// BillingIntervalMonth represents monthly recurring billing.
	BillingIntervalMonth BillingInterval = "month"
	// BillingIntervalYear represents annual recurring billing.
	BillingIntervalYear BillingInterval = "year"
)

const (
	// BillingStatusFree represents an account on the free tier.
	BillingStatusFree BillingStatus = "free"
	// BillingStatusCheckoutPending represents a checkout session initiated but not yet finalized.
	BillingStatusCheckoutPending BillingStatus = "checkout_pending"
	// BillingStatusActive represents an active subscription in good standing.
	BillingStatusActive BillingStatus = "active"
	// BillingStatusPastDue indicates payment failure on a renewal attempt.
	BillingStatusPastDue BillingStatus = "past_due"
	// BillingStatusUnpaid indicates an unpaid subscription awaiting settlement.
	BillingStatusUnpaid BillingStatus = "unpaid"
	// BillingStatusCanceled indicates a canceled subscription.
	BillingStatusCanceled BillingStatus = "canceled"
	// BillingStatusIncomplete indicates an initial payment failure on checkout.
	BillingStatusIncomplete BillingStatus = "incomplete"
)

// Validate ensures the billing plan is one of the supported plan variants.
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

// Validate ensures the billing currency is supported.
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

// Validate checks whether the billing interval is valid.
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

// BillingCheckoutSession contains the redirect URL for a Stripe Checkout session.
type BillingCheckoutSession struct {
	URL string `json:"url"`
}

// BillingPortalSession contains the redirect URL for the Stripe Customer Portal.
type BillingPortalSession struct {
	URL string `json:"url"`
}

// BillingCustomerRef contains an account's external payment processor customer identifier.
type BillingCustomerRef struct {
	ExternalCustomerID string `json:"external_customer_id"`
}

// ProviderWebhookEvent contains normalized event data parsed from payment provider webhooks.
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

	// The following are side-effect payloads for syncing cached invoices and payment methods.
	// Processed independently from account billing state updates (only non-nil entries applied).
	Invoice              *ProviderInvoice       `json:"invoice,omitempty"`
	PaymentMethod        *ProviderPaymentMethod `json:"payment_method,omitempty"`
	PaymentMethodDeleted string                 `json:"payment_method_deleted,omitempty"`
	// DefaultPaymentMethodSet indicates that customer.updated should trigger recomputation of default PM,
	// even if DefaultPaymentMethod is empty (clearing the default).
	DefaultPaymentMethod    string `json:"default_payment_method,omitempty"`
	DefaultPaymentMethodSet bool   `json:"default_payment_method_set,omitempty"`
}

// ProviderInvoice contains normalized invoice line details extracted from Stripe invoice.* webhooks.
// Used for upserting into the invoices table; account is resolved via ExternalCustomerID.
type ProviderInvoice struct {
	StripeInvoiceID  string `json:"stripe_invoice_id"`
	AmountMinor      int64  `json:"amount_minor"`
	Currency         string `json:"currency"`
	Status           string `json:"status"`
	HostedInvoiceURL string `json:"hosted_invoice_url"`
	InvoicePDFURL    string `json:"invoice_pdf_url"`
	PeriodStart      string `json:"period_start,omitempty"`
	PeriodEnd        string `json:"period_end,omitempty"`
	PaidAt           string `json:"paid_at,omitempty"`
	CreatedAt        string `json:"created_at"`
}

// ProviderPaymentMethod represents credit card metadata extracted from Stripe payment_method.* webhooks.
type ProviderPaymentMethod struct {
	StripePaymentMethodID string `json:"stripe_payment_method_id"`
	Brand                 string `json:"brand"`
	Last4                 string `json:"last4"`
	ExpMonth              int32  `json:"exp_month"`
	ExpYear               int32  `json:"exp_year"`
}

// =========================================================
// Usage-Based Billing (Phase 1-3)
// =========================================================

// PaidVia classifies how a metered usage event was settled:
//   - credit: fully deducted from account promotional/free credits (does not route to Stripe)
//   - stripe: fully billed to Stripe metered billing
//   - mixed: boundary event partially covered by remaining credit balance and remainder billed to Stripe
type PaidVia string

const (
	// PaidViaCredit indicates the event was settled entirely using promotional/grant credit balance.
	PaidViaCredit PaidVia = "credit"
	// PaidViaStripe indicates the event was billed to Stripe usage-based meters.
	PaidViaStripe PaidVia = "stripe"
	// PaidViaMixed indicates a boundary event split between credit deduction and Stripe meter billing.
	PaidViaMixed PaidVia = "mixed"
)

// UsageEvent represents a single metered LLM token consumption record.
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

// UsageRecordResult contains the outcome of recording and metering a usage event.
type UsageRecordResult struct {
	EventID           string  `json:"event_id"`
	Cost              string  `json:"cost"`
	BudgetExceeded    bool    `json:"budget_exceeded"`
	CreditStopped     bool    `json:"credit_stopped"` // true when execution stops due to exhausted free credits
	PaidVia           PaidVia `json:"paid_via,omitempty"`
	CreditAmountMinor int64   `json:"credit_amount_minor,omitempty"`
	StripeAmountMinor int64   `json:"stripe_amount_minor,omitempty"`
	// PricingMissing is true when model pricing could not be determined and cost was recorded as 0.
	// Indicates an uncaptured revenue leak rather than intentional free usage.
	PricingMissing bool `json:"pricing_missing,omitempty"`
}

// ModelUsage aggregates token volumes and costs for a specific AI model.
type ModelUsage struct {
	Model        string `json:"model"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	InputCost    string `json:"input_cost"`
	OutputCost   string `json:"output_cost"`
	TotalCost    string `json:"total_cost"`
	EventCount   int64  `json:"event_count"`
}

// DailyUsage aggregates daily cost totals.
type DailyUsage struct {
	Date      string `json:"date"`
	TotalCost string `json:"total_cost"`
}

// UsageReport compiles comprehensive usage metrics across models and days for an account.
type UsageReport struct {
	AccountID   string       `json:"account_id"`
	PeriodStart string       `json:"period_start"`
	PeriodEnd   string       `json:"period_end"`
	TotalCost   string       `json:"total_cost"`
	Currency    string       `json:"currency"`
	ByModel     []ModelUsage `json:"by_model"`
	ByDay       []DailyUsage `json:"by_day"`
}

// Invoice represents a finalized customer invoice record.
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

// InvoiceList provides a paginated collection of past invoices along with upcoming estimated charges.
type InvoiceList struct {
	Invoices          []*Invoice `json:"invoices"`
	UpcomingAmount    string     `json:"upcoming_amount"`
	UpcomingPeriodEnd string     `json:"upcoming_period_end"`
}

// ModelPricing defines per-million token input and output rates for an AI model.
type ModelPricing struct {
	Model                    string  `json:"model"`
	InputCostPerMTokenMinor  int64   `json:"input_cost_per_mtoken_minor"`
	OutputCostPerMTokenMinor int64   `json:"output_cost_per_mtoken_minor"`
	Currency                 string  `json:"currency"`
	DisplayMultiplier        float64 `json:"display_multiplier"`
}

// CreditType classifies the source or consumption status of account credits.
type CreditType string

const (
	// CreditTypeFree represents promotional or welcome signup credits.
	CreditTypeFree CreditType = "free"
	// CreditTypePurchased represents prepaid credits purchased by the customer.
	CreditTypePurchased CreditType = "purchased"
	// CreditTypeConsumed represents credit deductions for metered usage.
	CreditTypeConsumed CreditType = "consumed"
)

// CreditGrant records an addition or adjustment to an account's credit balance.
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
	// FreeSignupCreditMinor is the promotional credit amount granted on initial account signup (in USD cents).
	// $1.00 = 100 cents.
	FreeSignupCreditMinor int64 = 100
)

// PaymentMethod describes a customer's stored credit or debit card payment method.
type PaymentMethod struct {
	PaymentMethodID string `json:"payment_method_id"`
	Brand           string `json:"brand"`
	Last4           string `json:"last4"`
	ExpMonth        int32  `json:"exp_month"`
	ExpYear         int32  `json:"exp_year"`
	IsDefault       bool   `json:"is_default"`
}
