package domain

import (
	"errors"
	"fmt"
)

var (
	ErrBillingProviderNotConfigured   = errors.New("billing provider is not configured")
	ErrBillingProviderMisconfigured   = errors.New("billing provider is misconfigured")
	ErrBillingPlanInvalid             = errors.New("billing plan is invalid")
	ErrBillingCurrencyUnsupported     = errors.New("billing currency is unsupported")
	ErrBillingWebhookSignatureInvalid = errors.New("billing webhook signature is invalid")

	// Usage-Based Billing (Phase 1-3)
	ErrBillingUsageEventInvalid = errors.New("usage event is missing required fields")
	ErrBillingBudgetInvalid     = errors.New("budget limit is invalid")
	ErrBillingBudgetExceeded    = errors.New("account budget has been exceeded")
)

type BillingPlan string
type BillingCurrency string
type BillingInterval string
type BillingStatus string

const (
	BillingPlanFree       BillingPlan = "free"
	BillingPlanUsageBased BillingPlan = "usage_based"
)

const (
	BillingCurrencyJPY BillingCurrency = "jpy"
	BillingCurrencyUSD BillingCurrency = "usd"
)

const (
	BillingIntervalMonth BillingInterval = "month"
	BillingIntervalYear  BillingInterval = "year"
)

const (
	BillingStatusFree            BillingStatus = "free"
	BillingStatusCheckoutPending BillingStatus = "checkout_pending"
	BillingStatusActive          BillingStatus = "active"
	BillingStatusPastDue         BillingStatus = "past_due"
	BillingStatusUnpaid          BillingStatus = "unpaid"
	BillingStatusCanceled        BillingStatus = "canceled"
	BillingStatusIncomplete      BillingStatus = "incomplete"
)

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

type BillingCheckoutSession struct {
	URL string `json:"url"`
}

type BillingPortalSession struct {
	URL string `json:"url"`
}

type BillingCustomerRef struct {
	ExternalCustomerID string `json:"external_customer_id"`
}

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

// PaidVia は usage event の決済区分。
//   - credit: 全額クレジット残高から消費（Stripe には流れない）
//   - stripe: 全額 Stripe usage-based meter で課金
//   - mixed:  クレジット残高分 + Stripe meter 分の両方が発生した境界 event
type PaidVia string

const (
	PaidViaCredit PaidVia = "credit"
	PaidViaStripe PaidVia = "stripe"
	PaidViaMixed  PaidVia = "mixed"
)

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

type UsageRecordResult struct {
	EventID           string  `json:"event_id"`
	Cost              string  `json:"cost"`
	BudgetExceeded    bool    `json:"budget_exceeded"`
	CreditStopped     bool    `json:"credit_stopped"` // 無料クレジット残高0で停止
	PaidVia           PaidVia `json:"paid_via,omitempty"`
	CreditAmountMinor int64   `json:"credit_amount_minor,omitempty"`
	StripeAmountMinor int64   `json:"stripe_amount_minor,omitempty"`
}

type ModelUsage struct {
	Model        string `json:"model"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	InputCost    string `json:"input_cost"`
	OutputCost   string `json:"output_cost"`
	TotalCost    string `json:"total_cost"`
	EventCount   int64  `json:"event_count"`
}

type DailyUsage struct {
	Date      string `json:"date"`
	TotalCost string `json:"total_cost"`
}

type UsageReport struct {
	AccountID   string        `json:"account_id"`
	PeriodStart string        `json:"period_start"`
	PeriodEnd   string        `json:"period_end"`
	TotalCost   string        `json:"total_cost"`
	Currency    string        `json:"currency"`
	ByModel     []ModelUsage  `json:"by_model"`
	ByDay       []DailyUsage  `json:"by_day"`
}

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

type InvoiceList struct {
	Invoices           []*Invoice `json:"invoices"`
	UpcomingAmount     string     `json:"upcoming_amount"`
	UpcomingPeriodEnd  string     `json:"upcoming_period_end"`
}

type ModelPricing struct {
	Model                    string  `json:"model"`
	InputCostPerMTokenMinor  int64   `json:"input_cost_per_mtoken_minor"`
	OutputCostPerMTokenMinor int64   `json:"output_cost_per_mtoken_minor"`
	Currency                 string  `json:"currency"`
	DisplayMultiplier        float64 `json:"display_multiplier"`
}

// CreditType は account_credits.credit_type の値。
type CreditType string

const (
	CreditTypeFree      CreditType = "free"
	CreditTypePurchased CreditType = "purchased"
	CreditTypeConsumed  CreditType = "consumed"
)

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
	// FreeSignupCreditMinor はサインアップ時に付与する無料クレジット額 (USD cents)。
	// $1.00 = 100 cents。
	FreeSignupCreditMinor int64 = 100
)

type PaymentMethod struct {
	PaymentMethodID string `json:"payment_method_id"`
	Brand           string `json:"brand"`
	Last4           string `json:"last4"`
	ExpMonth        int32  `json:"exp_month"`
	ExpYear         int32  `json:"exp_year"`
	IsDefault       bool   `json:"is_default"`
}
