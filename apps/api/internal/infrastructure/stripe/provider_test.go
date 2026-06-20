package stripe

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/api/internal/domain"
)

func TestParseWebhook_CheckoutSessionCompleted(t *testing.T) {
	now := time.Unix(1710000000, 0)
	payload := []byte(`{
		"id": "evt_1",
		"type": "checkout.session.completed",
		"data": {
			"object": {
				"customer": "cus_1",
				"subscription": "sub_1",
				"metadata": {"account_id": "acc_1"}
			}
		}
	}`)
	provider := &Provider{
		cfg: Config{WebhookSecret: "whsec_test"},
		now: func() time.Time {
			return now
		},
	}

	event, err := provider.ParseWebhook(t.Context(), payload, stripeSignature("whsec_test", now, payload))

	require.NoError(t, err)
	assert.Equal(t, "evt_1", event.EventID)
	assert.Equal(t, "checkout.session.completed", event.EventType)
	assert.Equal(t, "acc_1", event.AccountID)
	assert.Equal(t, "cus_1", event.ExternalCustomerID)
	assert.Equal(t, "sub_1", event.ExternalSubscriptionID)
	assert.Empty(t, event.Plan)
	assert.Equal(t, domain.BillingStatusCheckoutPending, event.Status)
}

func TestParseWebhook_InvalidSignature(t *testing.T) {
	now := time.Unix(1710000000, 0)
	provider := &Provider{
		cfg: Config{WebhookSecret: "whsec_test"},
		now: func() time.Time {
			return now
		},
	}

	_, err := provider.ParseWebhook(t.Context(), []byte(`{"id":"evt_1"}`), "t=1710000000,v1=bad")

	require.ErrorIs(t, err, domain.ErrBillingWebhookSignatureInvalid)
}

func TestParseWebhook_InvoicePaidUsesSubscriptionIDNotInvoiceID(t *testing.T) {
	now := time.Unix(1710000000, 0)
	payload := []byte(`{
		"id": "evt_invoice",
		"type": "invoice.paid",
		"data": {
			"object": {
				"id": "in_1",
				"customer": "cus_1",
				"subscription": "sub_1",
				"subscription_details": {"metadata": {"account_id": "acc_1"}},
				"lines": {"data": [{"price": {"id": "price_jpy", "unit_amount": 1000, "recurring": {"interval": "month"}}}]}
			}
		}
	}`)
	provider := &Provider{
		cfg: Config{WebhookSecret: "whsec_test"},
		pricesByID: map[string]Price{
			"price_jpy": {Plan: domain.BillingPlanUsageBased, Currency: domain.BillingCurrencyJPY, Interval: domain.BillingIntervalMonth, StripeID: "price_jpy"},
		},
		now: func() time.Time { return now },
	}

	event, err := provider.ParseWebhook(t.Context(), payload, stripeSignature("whsec_test", now, payload))

	require.NoError(t, err)
	assert.Equal(t, "sub_1", event.ExternalSubscriptionID)
	assert.Equal(t, "price_jpy", event.ExternalPriceID)
	assert.Equal(t, domain.BillingPlanUsageBased, event.Plan)
}

func TestParseWebhook_UnknownPriceHasNoPlan(t *testing.T) {
	now := time.Unix(1710000000, 0)
	payload := []byte(`{
		"id": "evt_sub",
		"type": "customer.subscription.updated",
		"data": {
			"object": {
				"id": "sub_1",
				"customer": "cus_1",
				"status": "active",
				"metadata": {"account_id": "acc_1"},
				"items": {"data": [{"price": {"id": "price_unknown", "unit_amount": 1000, "recurring": {"interval": "month"}}}]}
			}
		}
	}`)
	provider := &Provider{
		cfg:        Config{WebhookSecret: "whsec_test"},
		pricesByID: map[string]Price{},
		now:        func() time.Time { return now },
	}

	event, err := provider.ParseWebhook(t.Context(), payload, stripeSignature("whsec_test", now, payload))

	require.NoError(t, err)
	assert.Empty(t, event.Plan)
	assert.Equal(t, "price_unknown", event.ExternalPriceID)
	assert.Equal(t, domain.BillingStatusActive, event.Status)
}

func TestParseWebhook_InvoicePaidExtractsInvoicePayload(t *testing.T) {
	now := time.Unix(1710000000, 0)
	payload := []byte(`{
		"id": "evt_invoice_paid",
		"type": "invoice.paid",
		"data": {
			"object": {
				"id": "in_42",
				"customer": "cus_1",
				"subscription": "sub_1",
				"currency": "USD",
				"amount_paid": 2500,
				"total": 2500,
				"hosted_invoice_url": "https://pay.stripe.com/in_42",
				"invoice_pdf": "https://pay.stripe.com/in_42.pdf",
				"created": 1709990000,
				"status_transitions": {"paid_at": 1709995000},
				"subscription_details": {"metadata": {"account_id": "acc_1"}},
				"lines": {"data": [{"price": {"id": "price_usd"}, "period": {"start": 1709990000, "end": 1712582000}}]}
			}
		}
	}`)
	provider := &Provider{
		cfg:        Config{WebhookSecret: "whsec_test"},
		pricesByID: map[string]Price{"price_usd": {Plan: domain.BillingPlanUsageBased, Currency: domain.BillingCurrencyUSD, StripeID: "price_usd"}},
		now:        func() time.Time { return now },
	}

	event, err := provider.ParseWebhook(t.Context(), payload, stripeSignature("whsec_test", now, payload))

	require.NoError(t, err)
	require.NotNil(t, event.Invoice)
	assert.Equal(t, "in_42", event.Invoice.StripeInvoiceID)
	assert.Equal(t, int64(2500), event.Invoice.AmountMinor)
	assert.Equal(t, "usd", event.Invoice.Currency)
	assert.Equal(t, "paid", event.Invoice.Status)
	assert.Equal(t, "https://pay.stripe.com/in_42", event.Invoice.HostedInvoiceURL)
	assert.Equal(t, "https://pay.stripe.com/in_42.pdf", event.Invoice.InvoicePDFURL)
	assert.Equal(t, time.Unix(1709990000, 0).UTC().Format(time.RFC3339), event.Invoice.PeriodStart)
	assert.Equal(t, time.Unix(1712582000, 0).UTC().Format(time.RFC3339), event.Invoice.PeriodEnd)
	assert.Equal(t, time.Unix(1709995000, 0).UTC().Format(time.RFC3339), event.Invoice.PaidAt)
	// account 状態更新も並走する
	assert.Equal(t, domain.BillingPlanUsageBased, event.Plan)
}

func TestParseWebhook_InvoiceFinalizedIsInvoiceOnly(t *testing.T) {
	now := time.Unix(1710000000, 0)
	payload := []byte(`{
		"id": "evt_finalized",
		"type": "invoice.finalized",
		"data": {
			"object": {
				"id": "in_open",
				"customer": "cus_1",
				"currency": "usd",
				"total": 1000,
				"amount_due": 1000,
				"created": 1709990000
			}
		}
	}`)
	provider := &Provider{cfg: Config{WebhookSecret: "whsec_test"}, now: func() time.Time { return now }}

	event, err := provider.ParseWebhook(t.Context(), payload, stripeSignature("whsec_test", now, payload))

	require.NoError(t, err)
	assert.Empty(t, event.Plan) // account 状態は触らない
	assert.Equal(t, "cus_1", event.ExternalCustomerID)
	require.NotNil(t, event.Invoice)
	assert.Equal(t, "in_open", event.Invoice.StripeInvoiceID)
	assert.Equal(t, "open", event.Invoice.Status)
	assert.Equal(t, int64(1000), event.Invoice.AmountMinor)
}

func TestParseWebhook_InvoiceVoided(t *testing.T) {
	now := time.Unix(1710000000, 0)
	payload := []byte(`{
		"id": "evt_void",
		"type": "invoice.voided",
		"data": {"object": {"id": "in_void", "customer": "cus_1", "currency": "usd", "total": 500}}
	}`)
	provider := &Provider{cfg: Config{WebhookSecret: "whsec_test"}, now: func() time.Time { return now }}

	event, err := provider.ParseWebhook(t.Context(), payload, stripeSignature("whsec_test", now, payload))

	require.NoError(t, err)
	require.NotNil(t, event.Invoice)
	assert.Equal(t, "void", event.Invoice.Status)
}

func TestParseWebhook_PaymentMethodAttached(t *testing.T) {
	now := time.Unix(1710000000, 0)
	payload := []byte(`{
		"id": "evt_pm",
		"type": "payment_method.attached",
		"data": {
			"object": {
				"id": "pm_1",
				"customer": "cus_1",
				"card": {"brand": "visa", "last4": "4242", "exp_month": 12, "exp_year": 2030}
			}
		}
	}`)
	provider := &Provider{cfg: Config{WebhookSecret: "whsec_test"}, now: func() time.Time { return now }}

	event, err := provider.ParseWebhook(t.Context(), payload, stripeSignature("whsec_test", now, payload))

	require.NoError(t, err)
	assert.Empty(t, event.Plan)
	assert.Equal(t, "cus_1", event.ExternalCustomerID)
	require.NotNil(t, event.PaymentMethod)
	assert.Equal(t, "pm_1", event.PaymentMethod.StripePaymentMethodID)
	assert.Equal(t, "visa", event.PaymentMethod.Brand)
	assert.Equal(t, "4242", event.PaymentMethod.Last4)
	assert.Equal(t, int32(12), event.PaymentMethod.ExpMonth)
	assert.Equal(t, int32(2030), event.PaymentMethod.ExpYear)
}

func TestParseWebhook_PaymentMethodDetached(t *testing.T) {
	now := time.Unix(1710000000, 0)
	payload := []byte(`{
		"id": "evt_pm_del",
		"type": "payment_method.detached",
		"data": {"object": {"id": "pm_1", "customer": null}}
	}`)
	provider := &Provider{cfg: Config{WebhookSecret: "whsec_test"}, now: func() time.Time { return now }}

	event, err := provider.ParseWebhook(t.Context(), payload, stripeSignature("whsec_test", now, payload))

	require.NoError(t, err)
	assert.Equal(t, "pm_1", event.PaymentMethodDeleted)
	assert.Nil(t, event.PaymentMethod)
}

func TestParseWebhook_CustomerUpdatedSetsDefaultPaymentMethod(t *testing.T) {
	now := time.Unix(1710000000, 0)
	payload := []byte(`{
		"id": "evt_cust",
		"type": "customer.updated",
		"data": {"object": {"id": "cus_1", "invoice_settings": {"default_payment_method": "pm_default"}}}
	}`)
	provider := &Provider{cfg: Config{WebhookSecret: "whsec_test"}, now: func() time.Time { return now }}

	event, err := provider.ParseWebhook(t.Context(), payload, stripeSignature("whsec_test", now, payload))

	require.NoError(t, err)
	assert.True(t, event.DefaultPaymentMethodSet)
	assert.Equal(t, "pm_default", event.DefaultPaymentMethod)
	// customer.updated の object は customer 本体なので顧客 ID は object.id から取る
	assert.Equal(t, "cus_1", event.ExternalCustomerID)
}

func stripeSignature(secret string, ts time.Time, payload []byte) string {
	timestamp := fmt.Sprintf("%d", ts.Unix())
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(payload)
	return fmt.Sprintf("t=%s,v1=%s", timestamp, hex.EncodeToString(mac.Sum(nil)))
}
