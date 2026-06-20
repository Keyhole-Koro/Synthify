package stripe

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/synthify/backend/apps/api/internal/domain"
)

type Config struct {
	SecretKey        string
	WebhookSecret    string
	ProPriceIDJPY    string
	ProPriceIDUSD    string
	DefaultCurrency  string
	SuccessURL       string
	CancelURL        string
	PortalReturnURL  string
	APIBase          string
	APIVersion       string
	MeterInputEvent  string
	MeterOutputEvent string
}

type Price struct {
	Plan        domain.BillingPlan
	Currency    domain.BillingCurrency
	Interval    domain.BillingInterval
	StripeID    string
	AmountMinor int64
}

type Provider struct {
	cfg             Config
	pricesByKey     map[string][]Price
	pricesByID      map[string]Price
	defaultCurrency domain.BillingCurrency
	client          *http.Client
	now             func() time.Time
}

func NewProvider(cfg Config) (*Provider, error) {
	if strings.TrimSpace(cfg.SecretKey) == "" {
		return nil, domain.ErrBillingProviderNotConfigured
	}
	pricesByKey, pricesByID, defaultCurrency, err := buildPriceCatalog(cfg)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.SuccessURL) == "" {
		return nil, fmt.Errorf("%w: BILLING_SUCCESS_URL is required", domain.ErrBillingProviderMisconfigured)
	}
	if strings.TrimSpace(cfg.CancelURL) == "" {
		return nil, fmt.Errorf("%w: BILLING_CANCEL_URL is required", domain.ErrBillingProviderMisconfigured)
	}
	if strings.TrimSpace(cfg.PortalReturnURL) == "" {
		return nil, fmt.Errorf("%w: BILLING_PORTAL_RETURN_URL is required", domain.ErrBillingProviderMisconfigured)
	}
	if cfg.APIBase == "" {
		cfg.APIBase = "https://api.stripe.com"
	}
	if cfg.APIVersion == "" {
		cfg.APIVersion = "2025-06-30.basil"
	}
	return &Provider{
		cfg:             cfg,
		pricesByKey:     pricesByKey,
		pricesByID:      pricesByID,
		defaultCurrency: defaultCurrency,
		client:          http.DefaultClient,
		now:             time.Now,
	}, nil
}

func buildPriceCatalog(cfg Config) (map[string][]Price, map[string]Price, domain.BillingCurrency, error) {
	defaultCurrency := domain.BillingCurrency(strings.ToLower(strings.TrimSpace(cfg.DefaultCurrency)))
	if defaultCurrency == "" {
		defaultCurrency = domain.BillingCurrencyJPY
	}
	if err := defaultCurrency.Validate(); err != nil {
		return nil, nil, "", err
	}
	type currencyGroup struct {
		currency domain.BillingCurrency
		raw      string
	}
	groups := []currencyGroup{
		{domain.BillingCurrencyJPY, cfg.ProPriceIDJPY},
		{domain.BillingCurrencyUSD, cfg.ProPriceIDUSD},
	}

	byKey := make(map[string][]Price)
	byID := make(map[string]Price)
	addPrice := func(currency domain.BillingCurrency, stripeID string) {
		price := Price{
			Plan:     domain.BillingPlanUsageBased,
			Currency: currency,
			Interval: domain.BillingIntervalMonth,
			StripeID: stripeID,
		}
		key := priceKey(price.Plan, price.Currency)
		byKey[key] = append(byKey[key], price)
		byID[stripeID] = price
	}

	for _, g := range groups {
		for _, id := range splitPriceIDs(g.raw) {
			addPrice(g.currency, id)
		}
	}
	if len(byKey) == 0 {
		return nil, nil, "", fmt.Errorf("%w: STRIPE_PRO_PRICE_ID_JPY or STRIPE_PRO_PRICE_ID_USD is required", domain.ErrBillingProviderMisconfigured)
	}
	return byKey, byID, defaultCurrency, nil
}

func splitPriceIDs(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if id := strings.TrimSpace(p); id != "" {
			out = append(out, id)
		}
	}
	return out
}

func priceKey(plan domain.BillingPlan, currency domain.BillingCurrency) string {
	return string(plan) + ":" + string(currency)
}

func (p *Provider) EnsureCustomer(ctx context.Context, account *domain.Account) (*domain.BillingCustomerRef, error) {
	if account.StripeCustomerID != "" {
		return &domain.BillingCustomerRef{ExternalCustomerID: account.StripeCustomerID}, nil
	}
	form := url.Values{}
	form.Set("name", account.Name)
	form.Set("metadata[account_id]", account.AccountID)
	var res struct {
		ID string `json:"id"`
	}
	if err := p.postForm(ctx, "/v1/customers", form, "customer:"+account.AccountID, &res); err != nil {
		return nil, err
	}
	if res.ID == "" {
		return nil, fmt.Errorf("%w: customer response missing id", domain.ErrBillingProviderMisconfigured)
	}
	return &domain.BillingCustomerRef{ExternalCustomerID: res.ID}, nil
}

func (p *Provider) CreateCheckoutSession(ctx context.Context, account *domain.Account, plan domain.BillingPlan, currency domain.BillingCurrency) (*domain.BillingCheckoutSession, error) {
	if plan != domain.BillingPlanUsageBased {
		return nil, fmt.Errorf("%w: %s", domain.ErrBillingPlanInvalid, plan)
	}
	if account.StripeCustomerID == "" {
		return nil, fmt.Errorf("%w: account has no Stripe customer", domain.ErrBillingProviderMisconfigured)
	}
	prices, err := p.resolvePrices(plan, currency)
	if err != nil {
		return nil, err
	}
	resolvedCurrency := prices[0].Currency
	form := url.Values{}
	form.Set("mode", "subscription")
	form.Set("customer", account.StripeCustomerID)
	for i, price := range prices {
		prefix := fmt.Sprintf("line_items[%d]", i)
		form.Set(prefix+"[price]", price.StripeID)
		// Metered prices ignore quantity; Stripe rejects "quantity" on them.
		// Skip the field entirely — flat-rate prices default to 1.
	}
	form.Set("success_url", p.cfg.SuccessURL)
	form.Set("cancel_url", p.cfg.CancelURL)
	form.Set("metadata[account_id]", account.AccountID)
	form.Set("metadata[currency]", string(resolvedCurrency))
	form.Set("subscription_data[metadata][account_id]", account.AccountID)
	form.Set("subscription_data[metadata][currency]", string(resolvedCurrency))
	form.Set("allow_promotion_codes", "true")
	var res struct {
		URL string `json:"url"`
	}
	if err := p.postForm(ctx, "/v1/checkout/sessions", form, "checkout:"+account.AccountID+":"+string(plan)+":"+string(resolvedCurrency), &res); err != nil {
		return nil, err
	}
	if res.URL == "" {
		return nil, fmt.Errorf("%w: checkout session response missing url", domain.ErrBillingProviderMisconfigured)
	}
	return &domain.BillingCheckoutSession{URL: res.URL}, nil
}

func (p *Provider) resolvePrices(plan domain.BillingPlan, currency domain.BillingCurrency) ([]Price, error) {
	if currency == "" {
		currency = p.defaultCurrency
	}
	if err := currency.Validate(); err != nil {
		return nil, err
	}
	prices, ok := p.pricesByKey[priceKey(plan, currency)]
	if !ok || len(prices) == 0 {
		return nil, fmt.Errorf("%w: %s", domain.ErrBillingCurrencyUnsupported, currency)
	}
	return prices, nil
}

func (p *Provider) CreatePortalSession(ctx context.Context, account *domain.Account) (*domain.BillingPortalSession, error) {
	if account.StripeCustomerID == "" {
		return nil, fmt.Errorf("%w: account has no Stripe customer", domain.ErrBillingProviderMisconfigured)
	}
	form := url.Values{}
	form.Set("customer", account.StripeCustomerID)
	form.Set("return_url", p.cfg.PortalReturnURL)
	var res struct {
		URL string `json:"url"`
	}
	if err := p.postForm(ctx, "/v1/billing_portal/sessions", form, "portal:"+account.AccountID+":"+strconv.FormatInt(p.now().Unix()/60, 10), &res); err != nil {
		return nil, err
	}
	if res.URL == "" {
		return nil, fmt.Errorf("%w: portal session response missing url", domain.ErrBillingProviderMisconfigured)
	}
	return &domain.BillingPortalSession{URL: res.URL}, nil
}

// ReportTokenUsage sends Stripe Billing meter events for LLM token usage.
// Input/output token counts go to separate meters configured via MeterInputEvent / MeterOutputEvent.
// identifier is used as idempotency key (suffixed per-side) so repeated calls are deduplicated.
// Silently no-ops when the account has no Stripe customer or meter events are not configured.
func (p *Provider) ReportTokenUsage(ctx context.Context, account *domain.Account, identifier string, inputTokens, outputTokens int64) error {
	if account == nil || account.StripeCustomerID == "" {
		return nil
	}
	if err := p.reportMeterEvent(ctx, account.StripeCustomerID, p.cfg.MeterInputEvent, inputTokens, identifier+":in"); err != nil {
		return err
	}
	return p.reportMeterEvent(ctx, account.StripeCustomerID, p.cfg.MeterOutputEvent, outputTokens, identifier+":out")
}

func (p *Provider) FetchBillingState(ctx context.Context, account *domain.Account) (*domain.ProviderWebhookEvent, error) {
	if account == nil || account.StripeCustomerID == "" {
		return nil, domain.ErrNotFound
	}
	params := url.Values{}
	params.Set("customer", account.StripeCustomerID)
	params.Set("status", "all")
	params.Set("limit", "1")
	var res struct {
		Data []stripeObject `json:"data"`
	}
	if err := p.getForm(ctx, "/v1/subscriptions", params, &res); err != nil {
		return nil, err
	}
	if len(res.Data) == 0 {
		return &domain.ProviderWebhookEvent{
			Provider:           "stripe",
			AccountID:          account.AccountID,
			ExternalCustomerID: account.StripeCustomerID,
			Plan:               domain.BillingPlanFree,
			Status:             domain.BillingStatusFree,
		}, nil
	}
	obj := res.Data[0]
	event := stripeEvent{ID: "reconcile:" + account.AccountID, Type: "customer.subscription.updated"}
	remote := p.providerEventFromPrice(event, obj, p.priceFromObject(obj), subscriptionStatus(obj.Status))
	if remote.AccountID == "" {
		remote.AccountID = account.AccountID
	}
	return remote, nil
}

// ListRemoteInvoices fetches recent invoices for the account's customer (reconcile backfill).
func (p *Provider) ListRemoteInvoices(ctx context.Context, account *domain.Account, limit int) ([]*domain.ProviderInvoice, error) {
	if account == nil || account.StripeCustomerID == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	params := url.Values{}
	params.Set("customer", account.StripeCustomerID)
	params.Set("limit", strconv.Itoa(limit))
	var res struct {
		Data []stripeObject `json:"data"`
	}
	if err := p.getForm(ctx, "/v1/invoices", params, &res); err != nil {
		return nil, err
	}
	out := make([]*domain.ProviderInvoice, 0, len(res.Data))
	for _, obj := range res.Data {
		out = append(out, p.invoiceFromObject(obj, invoiceStatusFromStripe(obj.Status)))
	}
	return out, nil
}

// ListRemotePaymentMethods fetches the account's saved cards plus the default payment
// method id (reconcile backfill). The default id comes from the customer object.
func (p *Provider) ListRemotePaymentMethods(ctx context.Context, account *domain.Account) ([]*domain.ProviderPaymentMethod, string, error) {
	if account == nil || account.StripeCustomerID == "" {
		return nil, "", nil
	}
	params := url.Values{}
	params.Set("customer", account.StripeCustomerID)
	params.Set("type", "card")
	var res struct {
		Data []stripeObject `json:"data"`
	}
	if err := p.getForm(ctx, "/v1/payment_methods", params, &res); err != nil {
		return nil, "", err
	}
	out := make([]*domain.ProviderPaymentMethod, 0, len(res.Data))
	for _, obj := range res.Data {
		out = append(out, paymentMethodFromObject(obj))
	}
	var customer stripeObject
	if err := p.getForm(ctx, "/v1/customers/"+account.StripeCustomerID, nil, &customer); err != nil {
		// default id は best-effort。取得失敗時は空（=全件 is_default=false）で返す。
		return out, "", nil
	}
	return out, customer.InvoiceSettings.DefaultPaymentMethod, nil
}

// invoiceStatusFromStripe maps Stripe's invoice.status to our cached status value.
func invoiceStatusFromStripe(status string) string {
	switch status {
	case "paid":
		return "paid"
	case "uncollectible":
		return "uncollectible"
	case "void":
		return "void"
	default:
		// draft / open はまとめて "open" 扱い。
		return "open"
	}
}

func (p *Provider) reportMeterEvent(ctx context.Context, customerID, eventName string, value int64, identifier string) error {
	if strings.TrimSpace(eventName) == "" || value <= 0 {
		return nil
	}
	form := url.Values{}
	form.Set("event_name", eventName)
	form.Set("payload[stripe_customer_id]", customerID)
	form.Set("payload[value]", strconv.FormatInt(value, 10))
	if identifier != "" {
		form.Set("identifier", identifier)
	}
	form.Set("timestamp", strconv.FormatInt(p.now().Unix(), 10))
	var res struct {
		Identifier string `json:"identifier"`
	}
	return p.postForm(ctx, "/v1/billing/meter_events", form, "meter:"+identifier, &res)
}

func (p *Provider) ParseWebhook(ctx context.Context, payload []byte, signature string) (*domain.ProviderWebhookEvent, error) {
	if strings.TrimSpace(p.cfg.WebhookSecret) == "" {
		return nil, fmt.Errorf("%w: STRIPE_WEBHOOK_SECRET is required", domain.ErrBillingProviderMisconfigured)
	}
	if err := p.verifySignature(payload, signature); err != nil {
		return nil, err
	}
	var event stripeEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}
	obj := event.Data.Object
	switch event.Type {
	case "checkout.session.completed":
		return &domain.ProviderWebhookEvent{
			Provider:               "stripe",
			EventID:                event.ID,
			EventType:              event.Type,
			AccountID:              metadataValue(obj.Metadata, "account_id"),
			ExternalCustomerID:     obj.Customer,
			ExternalSubscriptionID: obj.Subscription,
			Status:                 domain.BillingStatusCheckoutPending,
		}, nil
	case "invoice.paid", "invoice.payment_succeeded":
		ev := p.providerEventFromPrice(event, obj, p.priceFromObject(obj), domain.BillingStatusActive)
		ev.Invoice = p.invoiceFromObject(obj, "paid")
		return ev, nil
	case "invoice.payment_failed":
		ev := p.providerEventFromPrice(event, obj, p.priceFromObject(obj), domain.BillingStatusPastDue)
		ev.Invoice = p.invoiceFromObject(obj, "open")
		return ev, nil
	case "invoice.finalized":
		return p.invoiceOnlyEvent(event, obj, "open"), nil
	case "invoice.marked_uncollectible":
		return p.invoiceOnlyEvent(event, obj, "uncollectible"), nil
	case "invoice.voided":
		return p.invoiceOnlyEvent(event, obj, "void"), nil
	case "payment_method.attached", "payment_method.automatically_updated":
		ev := p.baseEvent(event, obj)
		ev.PaymentMethod = paymentMethodFromObject(obj)
		return ev, nil
	case "payment_method.detached":
		ev := p.baseEvent(event, obj)
		ev.PaymentMethodDeleted = obj.ID
		return ev, nil
	case "customer.updated":
		// customer.updated の object は customer 本体なので顧客 ID は obj.ID。
		return &domain.ProviderWebhookEvent{
			Provider:                "stripe",
			EventID:                 event.ID,
			EventType:               event.Type,
			ExternalCustomerID:      obj.ID,
			DefaultPaymentMethodSet: true,
			DefaultPaymentMethod:    obj.InvoiceSettings.DefaultPaymentMethod,
		}, nil
	case "customer.subscription.created", "customer.subscription.updated":
		return p.providerEventFromPrice(event, obj, p.priceFromObject(obj), subscriptionStatus(obj.Status)), nil
	case "customer.subscription.deleted":
		return &domain.ProviderWebhookEvent{
			Provider:               "stripe",
			EventID:                event.ID,
			EventType:              event.Type,
			AccountID:              metadataValue(obj.Metadata, "account_id"),
			ExternalCustomerID:     obj.Customer,
			ExternalSubscriptionID: "",
			Plan:                   domain.BillingPlanFree,
			Status:                 domain.BillingStatusFree,
		}, nil
	default:
		return &domain.ProviderWebhookEvent{
			Provider:  "stripe",
			EventID:   event.ID,
			EventType: event.Type,
		}, nil
	}
}

func (p *Provider) providerEventFromPrice(event stripeEvent, obj stripeObject, price Price, status domain.BillingStatus) *domain.ProviderWebhookEvent {
	subscriptionID := firstNonEmpty(obj.Subscription, obj.Parent.SubscriptionDetails.Subscription)
	if strings.HasPrefix(event.Type, "customer.subscription.") {
		subscriptionID = obj.ID
	}
	return &domain.ProviderWebhookEvent{
		Provider:               "stripe",
		EventID:                event.ID,
		EventType:              event.Type,
		AccountID:              firstNonEmpty(metadataValue(obj.Metadata, "account_id"), metadataValue(obj.SubscriptionDetails.Metadata, "account_id")),
		ExternalCustomerID:     obj.Customer,
		ExternalSubscriptionID: subscriptionID,
		Plan:                   price.Plan,
		Status:                 status,
		ExternalPriceID:        price.StripeID,
		Currency:               price.Currency,
		AmountMinor:            price.AmountMinor,
		Interval:               price.Interval,
		CurrentPeriodEnd:       unixToRFC3339(obj.CurrentPeriodEnd),
		CancelAtPeriodEnd:      obj.CancelAtPeriodEnd,
	}
}

// baseEvent builds an event carrying only identity + customer, for side-effect-only
// webhooks (payment_method.*) that must not touch the account billing state.
func (p *Provider) baseEvent(event stripeEvent, obj stripeObject) *domain.ProviderWebhookEvent {
	return &domain.ProviderWebhookEvent{
		Provider:           "stripe",
		EventID:            event.ID,
		EventType:          event.Type,
		AccountID:          metadataValue(obj.Metadata, "account_id"),
		ExternalCustomerID: obj.Customer,
	}
}

func (p *Provider) invoiceOnlyEvent(event stripeEvent, obj stripeObject, status string) *domain.ProviderWebhookEvent {
	ev := p.baseEvent(event, obj)
	ev.Invoice = p.invoiceFromObject(obj, status)
	return ev
}

// invoiceFromObject maps a Stripe invoice object to a ProviderInvoice.
// status は webhook 種別ごとに呼び出し側がマップした値 (paid/open/uncollectible/void)。
func (p *Provider) invoiceFromObject(obj stripeObject, status string) *domain.ProviderInvoice {
	amount := obj.AmountPaid
	if amount == 0 {
		amount = firstNonZero(obj.Total, obj.AmountDue)
	}
	start, end := obj.Lines.Data.firstPeriod()
	return &domain.ProviderInvoice{
		StripeInvoiceID:  obj.ID,
		AmountMinor:      amount,
		Currency:         strings.ToLower(obj.Currency),
		Status:           status,
		HostedInvoiceURL: obj.HostedInvoiceURL,
		InvoicePDFURL:    obj.InvoicePDF,
		PeriodStart:      unixToRFC3339(start),
		PeriodEnd:        unixToRFC3339(end),
		PaidAt:           unixToRFC3339(obj.StatusTransitions.PaidAt),
		CreatedAt:        unixToRFC3339(obj.Created),
	}
}

func paymentMethodFromObject(obj stripeObject) *domain.ProviderPaymentMethod {
	return &domain.ProviderPaymentMethod{
		StripePaymentMethodID: obj.ID,
		Brand:                 obj.Card.Brand,
		Last4:                 obj.Card.Last4,
		ExpMonth:              obj.Card.ExpMonth,
		ExpYear:               obj.Card.ExpYear,
	}
}

func (p *Provider) priceFromObject(obj stripeObject) Price {
	priceID := firstNonEmpty(obj.Items.Data.firstPriceID(), obj.Lines.Data.firstPriceID())
	price, ok := p.pricesByID[priceID]
	if !ok {
		return Price{StripeID: priceID}
	}
	if price.AmountMinor == 0 {
		price.AmountMinor = firstNonZero(obj.Items.Data.firstUnitAmount(), obj.Lines.Data.firstUnitAmount())
	}
	return price
}

func (p *Provider) postForm(ctx context.Context, path string, form url.Values, idempotencyKey string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(p.cfg.APIBase, "/")+path, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth(p.cfg.SecretKey, "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Stripe-Version", p.cfg.APIVersion)
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return stripeError(resp.StatusCode, body)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return err
	}
	return nil
}

func (p *Provider) getForm(ctx context.Context, path string, params url.Values, out any) error {
	u := strings.TrimRight(p.cfg.APIBase, "/") + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(p.cfg.SecretKey, "")
	req.Header.Set("Stripe-Version", p.cfg.APIVersion)
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return stripeError(resp.StatusCode, body)
	}
	return json.Unmarshal(body, out)
}

func (p *Provider) verifySignature(payload []byte, header string) error {
	timestamp, signatures := parseSignatureHeader(header)
	if timestamp == "" || len(signatures) == 0 {
		return domain.ErrBillingWebhookSignatureInvalid
	}
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return domain.ErrBillingWebhookSignatureInvalid
	}
	signedAt := time.Unix(seconds, 0)
	if skew := p.now().Sub(signedAt); skew > 5*time.Minute || skew < -5*time.Minute {
		return domain.ErrBillingWebhookSignatureInvalid
	}
	mac := hmac.New(sha256.New, []byte(p.cfg.WebhookSecret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(payload)
	expected := mac.Sum(nil)
	for _, sig := range signatures {
		actual, err := hex.DecodeString(sig)
		if err == nil && hmac.Equal(actual, expected) {
			return nil
		}
	}
	return domain.ErrBillingWebhookSignatureInvalid
}

func parseSignatureHeader(header string) (string, []string) {
	var timestamp string
	var signatures []string
	for _, part := range strings.Split(header, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch key {
		case "t":
			timestamp = value
		case "v1":
			signatures = append(signatures, value)
		}
	}
	return timestamp, signatures
}

func stripeError(status int, body []byte) error {
	var res struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &res); err == nil && res.Error.Message != "" {
		return fmt.Errorf("stripe api error status=%d type=%s: %s", status, res.Error.Type, res.Error.Message)
	}
	return fmt.Errorf("stripe api error status=%d: %s", status, string(bytes.TrimSpace(body)))
}

type stripeEvent struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Data struct {
		Object stripeObject `json:"object"`
	} `json:"data"`
}

type stripeObject struct {
	ID                  string            `json:"id"`
	Customer            string            `json:"customer"`
	Subscription        string            `json:"subscription"`
	Status              string            `json:"status"`
	Metadata            map[string]string `json:"metadata"`
	CurrentPeriodEnd    int64             `json:"current_period_end"`
	CancelAtPeriodEnd   bool              `json:"cancel_at_period_end"`
	SubscriptionDetails struct {
		Metadata map[string]string `json:"metadata"`
	} `json:"subscription_details"`
	Items  stripeList `json:"items"`
	Lines  stripeList `json:"lines"`
	Parent struct {
		SubscriptionDetails struct {
			Subscription string `json:"subscription"`
		} `json:"subscription_details"`
	} `json:"parent"`

	// invoice.* イベント用フィールド
	Currency          string `json:"currency"`
	HostedInvoiceURL  string `json:"hosted_invoice_url"`
	InvoicePDF        string `json:"invoice_pdf"`
	AmountPaid        int64  `json:"amount_paid"`
	AmountDue         int64  `json:"amount_due"`
	Total             int64  `json:"total"`
	Created           int64  `json:"created"`
	StatusTransitions struct {
		PaidAt int64 `json:"paid_at"`
	} `json:"status_transitions"`

	// payment_method.* イベント用フィールド
	Card struct {
		Brand    string `json:"brand"`
		Last4    string `json:"last4"`
		ExpMonth int32  `json:"exp_month"`
		ExpYear  int32  `json:"exp_year"`
	} `json:"card"`

	// customer.updated イベント用フィールド（default PM は通常 string id で届く）
	InvoiceSettings struct {
		DefaultPaymentMethod string `json:"default_payment_method"`
	} `json:"invoice_settings"`
}

type stripeList struct {
	Data stripeLineItems `json:"data"`
}

type stripeLineItems []stripeLineItem

type stripeLineItem struct {
	Price  stripePrice `json:"price"`
	Period struct {
		Start int64 `json:"start"`
		End   int64 `json:"end"`
	} `json:"period"`
}

type stripePrice struct {
	ID         string `json:"id"`
	Currency   string `json:"currency"`
	UnitAmount int64  `json:"unit_amount"`
	Recurring  struct {
		Interval string `json:"interval"`
	} `json:"recurring"`
}

func (items stripeLineItems) firstPriceID() string {
	for _, item := range items {
		if item.Price.ID != "" {
			return item.Price.ID
		}
	}
	return ""
}

func (items stripeLineItems) firstUnitAmount() int64 {
	for _, item := range items {
		if item.Price.UnitAmount > 0 {
			return item.Price.UnitAmount
		}
	}
	return 0
}

func (items stripeLineItems) firstPeriod() (int64, int64) {
	for _, item := range items {
		if item.Period.Start != 0 || item.Period.End != 0 {
			return item.Period.Start, item.Period.End
		}
	}
	return 0, 0
}

func metadataValue(metadata map[string]string, key string) string {
	if metadata == nil {
		return ""
	}
	return metadata[key]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstNonZero(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func unixToRFC3339(seconds int64) string {
	if seconds == 0 {
		return ""
	}
	return time.Unix(seconds, 0).UTC().Format(time.RFC3339)
}

func subscriptionStatus(status string) domain.BillingStatus {
	switch status {
	case "active", "trialing":
		return domain.BillingStatusActive
	case "past_due":
		return domain.BillingStatusPastDue
	case "unpaid":
		return domain.BillingStatusUnpaid
	case "canceled":
		return domain.BillingStatusCanceled
	case "incomplete", "incomplete_expired":
		return domain.BillingStatusIncomplete
	default:
		return ""
	}
}
