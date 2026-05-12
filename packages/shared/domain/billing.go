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
)

type BillingPlan string
type BillingCurrency string
type BillingInterval string
type BillingStatus string

const (
	BillingPlanFree BillingPlan = "free"
	BillingPlanPro  BillingPlan = "pro"
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
	case BillingPlanFree, BillingPlanPro:
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
