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
	"github.com/synthify/backend/packages/shared/domain"
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

func stripeSignature(secret string, ts time.Time, payload []byte) string {
	timestamp := fmt.Sprintf("%d", ts.Unix())
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(payload)
	return fmt.Sprintf("t=%s,v1=%s", timestamp, hex.EncodeToString(mac.Sum(nil)))
}
