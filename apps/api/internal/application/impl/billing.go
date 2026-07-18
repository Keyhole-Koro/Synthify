package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository"
	platformusage "github.com/synthify/backend/internal/platform/billing/usage"
)

type BillingUsecase interface {
	GetBillingAccount(ctx context.Context, accountID, actorUserID string) (*domain.Account, error)
	CreateCheckoutSession(ctx context.Context, accountID, actorUserID string, plan domain.BillingPlan, currency domain.BillingCurrency) (*domain.BillingCheckoutSession, error)
	CreatePortalSession(ctx context.Context, accountID, actorUserID string) (*domain.BillingPortalSession, error)
	HandleWebhook(ctx context.Context, payload []byte, signature string) error

	GetUsage(ctx context.Context, accountID, actorUserID string, periodStart, periodEnd string) (*domain.UsageReport, error)
	RecordUsage(ctx context.Context, ev *domain.UsageEvent) (*domain.UsageRecordResult, error)
	UpdateBudget(ctx context.Context, accountID, actorUserID string, budgetLimit string) (string, error)
	ListInvoices(ctx context.Context, accountID, actorUserID string, limit int) (*domain.InvoiceList, error)
	ListPaymentMethods(ctx context.Context, accountID, actorUserID string) ([]*domain.PaymentMethod, error)

	// Credits
	GrantFreeSignupCredit(ctx context.Context, accountID string) error
	GrantCredit(ctx context.Context, actorUserID, accountID string, amountMinor int64, note string) (*domain.CreditGrant, error)
	GetCreditBalance(ctx context.Context, accountID, actorUserID string) (int64, error)
}

type BillingProvider interface {
	EnsureCustomer(ctx context.Context, account *domain.Account) (*domain.BillingCustomerRef, error)
	CreateCheckoutSession(ctx context.Context, account *domain.Account, plan domain.BillingPlan, currency domain.BillingCurrency) (*domain.BillingCheckoutSession, error)
	CreatePortalSession(ctx context.Context, account *domain.Account) (*domain.BillingPortalSession, error)
	ParseWebhook(ctx context.Context, payload []byte, signature string) (*domain.ProviderWebhookEvent, error)
	ReportTokenUsage(ctx context.Context, account *domain.Account, identifier string, inputTokens, outputTokens int64) error
	FetchBillingState(ctx context.Context, account *domain.Account) (*domain.ProviderWebhookEvent, error)
	// reconcile バックフィル用: webhook を取りこぼした過去分の invoice / payment method を
	// Stripe から引き直す。ListRemotePaymentMethods の第2戻り値は default payment method id。
	ListRemoteInvoices(ctx context.Context, account *domain.Account, limit int) ([]*domain.ProviderInvoice, error)
	ListRemotePaymentMethods(ctx context.Context, account *domain.Account) ([]*domain.ProviderPaymentMethod, string, error)
}

type BillingReconciler interface {
	ReconcileAccount(ctx context.Context, accountID string, apply bool) (*BillingReconciliationDiff, error)
	ReconcileLinkedAccounts(ctx context.Context, apply bool, limit int) ([]*BillingReconciliationDiff, error)
}

type billingService struct {
	accounts repository.AccountRepository
	usage    repository.UsageRepository
	provider BillingProvider
	logger   *slog.Logger
	now      func() time.Time
	recorder *platformusage.Recorder
}

type BillingServiceDeps struct {
	Accounts repository.AccountRepository
	// Usage may be nil during early local dev (before the postgres-backed
	// implementation lands); RecordUsage will then fall back to a logging-only
	// stub so the worker pipeline keeps running.
	Usage    repository.UsageRepository
	Provider BillingProvider
	Logger   *slog.Logger
	// RequireUsage makes a nil Usage repository a hard error instead of the
	// logging stub. Production/staging set this so a deploy that would bill
	// nothing fails fast at startup rather than silently leaking revenue.
	RequireUsage bool
}

// ErrBillingUsageRepoRequired is returned by NewBillingService when
// RequireUsage is set but no usage repository was provided. RecordUsage would
// otherwise fall back to a no-op stub that bills nothing.
var ErrBillingUsageRepoRequired = errors.New("billing: usage repository is required but not configured")

func NewBillingService(deps BillingServiceDeps) (BillingUsecase, error) {
	if deps.RequireUsage && deps.Usage == nil {
		return nil, ErrBillingUsageRepoRequired
	}
	now := time.Now
	s := &billingService{
		accounts: deps.Accounts,
		usage:    deps.Usage,
		provider: deps.Provider,
		logger:   deps.Logger,
		now:      now,
	}
	if deps.Usage != nil {
		adapter := newUsageRepoAdapter(deps.Usage, deps.Accounts)
		s.recorder = platformusage.NewRecorder(adapter, billingClock{now: now}, deps.Logger)
	}
	return s, nil
}

type billingClock struct{ now func() time.Time }

// NowRFC3339Date returns the current instant as an RFC3339 string. The
// recorder slices the first 10 chars to get YYYY-MM-DD for the daily
// rollup, and uses the full string for credit grant timestamps.
func (c billingClock) NowRFC3339Date() string {
	return c.now().UTC().Format("2006-01-02T15:04:05Z")
}

func (s *billingService) GetBillingAccount(ctx context.Context, accountID, actorUserID string) (*domain.Account, error) {
	account, err := s.authorizeAccount(ctx, accountID, actorUserID)
	if err != nil {
		s.logAuthorizeError(ctx, "billing.get_account.authorize_failed", err,
			"account_id", accountID,
			"actor_user_id", actorUserID,
		)
		return nil, err
	}
	return account, nil
}

func (s *billingService) CreateCheckoutSession(ctx context.Context, accountID, actorUserID string, plan domain.BillingPlan, currency domain.BillingCurrency) (*domain.BillingCheckoutSession, error) {
	if err := plan.Validate(); err != nil {
		s.logger.Warn("billing.checkout_session.invalid_plan",
			"error", err.Error(),
			"account_id", accountID,
			"actor_user_id", actorUserID,
			"plan", plan,
		)
		return nil, err
	}
	if currency != "" {
		if err := currency.Validate(); err != nil {
			s.logger.Warn("billing.checkout_session.invalid_currency",
				"error", err.Error(),
				"account_id", accountID,
				"actor_user_id", actorUserID,
				"currency", currency,
			)
			return nil, err
		}
	}
	account, err := s.authorizeAccount(ctx, accountID, actorUserID)
	if err != nil {
		s.logAuthorizeError(ctx, "billing.checkout_session.authorize_failed", err,
			"account_id", accountID,
			"actor_user_id", actorUserID,
			"plan", plan,
		)
		return nil, err
	}
	if s.provider == nil {
		s.noticeError(ctx, "billing.provider_not_configured", domain.ErrBillingProviderNotConfigured, map[string]any{
			"account_id": accountID,
			"operation":  "create_checkout_session",
		})
		s.logger.Error("billing.provider_not_configured",
			"error", domain.ErrBillingProviderNotConfigured.Error(),
			"account_id", accountID,
			"actor_user_id", actorUserID,
			"operation", "create_checkout_session",
		)
		return nil, domain.ErrBillingProviderNotConfigured
	}
	if err := s.ensureProviderCustomer(ctx, account); err != nil {
		s.noticeError(ctx, "billing.checkout_session.customer_failed", err, map[string]any{
			"account_id": accountID,
		})
		s.logger.Error("billing.checkout_session.customer_failed",
			"error", err.Error(),
			"account_id", accountID,
			"actor_user_id", actorUserID,
		)
		return nil, err
	}
	session, err := s.provider.CreateCheckoutSession(ctx, account, plan, currency)
	if err != nil {
		s.noticeError(ctx, "billing.checkout_session.provider_failed", err, map[string]any{
			"account_id": accountID,
			"plan":       string(plan),
			"currency":   string(currency),
		})
		s.logger.Error("billing.checkout_session.provider_failed",
			"error", err.Error(),
			"account_id", accountID,
			"actor_user_id", actorUserID,
			"plan", plan,
			"currency", currency,
		)
		return nil, err
	}
	s.logger.Info("billing.checkout_session.created",
		"account_id", accountID,
		"actor_user_id", actorUserID,
		"plan", plan,
		"currency", currency,
	)
	return session, nil
}

func (s *billingService) CreatePortalSession(ctx context.Context, accountID, actorUserID string) (*domain.BillingPortalSession, error) {
	account, err := s.authorizeAccount(ctx, accountID, actorUserID)
	if err != nil {
		s.logAuthorizeError(ctx, "billing.portal_session.authorize_failed", err,
			"account_id", accountID,
			"actor_user_id", actorUserID,
		)
		return nil, err
	}
	if s.provider == nil {
		s.noticeError(ctx, "billing.provider_not_configured", domain.ErrBillingProviderNotConfigured, map[string]any{
			"account_id": accountID,
			"operation":  "create_portal_session",
		})
		s.logger.Error("billing.provider_not_configured",
			"error", domain.ErrBillingProviderNotConfigured.Error(),
			"account_id", accountID,
			"actor_user_id", actorUserID,
			"operation", "create_portal_session",
		)
		return nil, domain.ErrBillingProviderNotConfigured
	}
	if err := s.ensureProviderCustomer(ctx, account); err != nil {
		s.noticeError(ctx, "billing.portal_session.customer_failed", err, map[string]any{
			"account_id": accountID,
		})
		s.logger.Error("billing.portal_session.customer_failed",
			"error", err.Error(),
			"account_id", accountID,
			"actor_user_id", actorUserID,
		)
		return nil, err
	}
	session, err := s.provider.CreatePortalSession(ctx, account)
	if err != nil {
		s.noticeError(ctx, "billing.portal_session.provider_failed", err, map[string]any{
			"account_id": accountID,
		})
		s.logger.Error("billing.portal_session.provider_failed",
			"error", err.Error(),
			"account_id", accountID,
			"actor_user_id", actorUserID,
		)
		return nil, err
	}
	s.logger.Info("billing.portal_session.created",
		"account_id", accountID,
		"actor_user_id", actorUserID,
	)
	return session, nil
}

func (s *billingService) HandleWebhook(ctx context.Context, payload []byte, signature string) error {
	if s.provider == nil {
		s.noticeError(ctx, "billing.provider_not_configured", domain.ErrBillingProviderNotConfigured, map[string]any{
			"operation": "handle_webhook",
		})
		s.logger.Error("billing.provider_not_configured",
			"error", domain.ErrBillingProviderNotConfigured.Error(),
			"operation", "handle_webhook",
		)
		return domain.ErrBillingProviderNotConfigured
	}
	event, err := s.provider.ParseWebhook(ctx, payload, signature)
	if err != nil {
		if errors.Is(err, domain.ErrBillingWebhookSignatureInvalid) {
			s.logger.Warn("billing.webhook.invalid_signature",
				"error", err.Error(),
				"payload_size", len(payload),
			)
			return err
		}
		s.noticeError(ctx, "billing.webhook.parse_failed", err, map[string]any{
			"payload_size": len(payload),
		})
		s.logger.Error("billing.webhook.parse_failed",
			"error", err.Error(),
			"payload_size", len(payload),
		)
		return err
	}
	applied, err := s.recordAndApplyWebhookEvent(ctx, event)
	if err != nil {
		s.noticeError(ctx, "billing.webhook.apply_failed", err, map[string]any{
			"event_id":   event.EventID,
			"event_type": event.EventType,
		})
		s.logger.Error("billing.webhook.apply_failed",
			"error", err.Error(),
			"event_id", event.EventID,
			"event_type", event.EventType,
			"account_id", event.AccountID,
			"external_customer_id", event.ExternalCustomerID,
			"external_subscription_id", event.ExternalSubscriptionID,
		)
		return err
	}
	s.logger.Info("billing.webhook.parsed",
		"payload_size", len(payload),
		"event_id", event.EventID,
		"event_type", event.EventType,
		"account_id", event.AccountID,
		"external_customer_id", event.ExternalCustomerID,
		"external_subscription_id", event.ExternalSubscriptionID,
		"applied", applied,
	)
	return err
}

func (s *billingService) ensureProviderCustomer(ctx context.Context, account *domain.Account) error {
	customer, err := s.provider.EnsureCustomer(ctx, account)
	if err != nil {
		return err
	}
	if customer == nil || customer.ExternalCustomerID == "" || customer.ExternalCustomerID == account.StripeCustomerID {
		return nil
	}
	if err := s.accounts.SetAccountStripeCustomerID(ctx, account.AccountID, customer.ExternalCustomerID); err != nil {
		return err
	}
	account.StripeCustomerID = customer.ExternalCustomerID
	return nil
}

func (s *billingService) recordAndApplyWebhookEvent(ctx context.Context, event *domain.ProviderWebhookEvent) (bool, error) {
	if event == nil {
		return false, nil
	}
	recorded, err := s.accounts.RecordBillingWebhookEvent(ctx, event)
	if err != nil {
		return false, err
	}
	if !recorded {
		s.logger.Info("billing.webhook.duplicate",
			"event_id", event.EventID,
			"event_type", event.EventType,
		)
		return false, nil
	}
	// account billing 状態を更新するのは plan が解決できたイベント、もしくは
	// checkout 完了の保留ステータスのみ。invoice/payment_method 専用イベントは
	// plan が空でも下記サイドエフェクトだけ走らせ、accounts は触らない。
	applyAccount := event.Plan != "" || event.Status == domain.BillingStatusCheckoutPending
	hasSideEffect := event.Invoice != nil || event.PaymentMethod != nil ||
		event.PaymentMethodDeleted != "" || event.DefaultPaymentMethodSet
	if !applyAccount && !hasSideEffect {
		if err := s.accounts.MarkBillingWebhookEventProcessed(ctx, event.Provider, event.EventID, "ignored", ""); err != nil {
			return false, err
		}
		return false, nil
	}
	if applyAccount {
		if event.Plan != "" {
			if err := event.Plan.Validate(); err != nil {
				_ = s.accounts.MarkBillingWebhookEventProcessed(ctx, event.Provider, event.EventID, "failed", err.Error())
				return false, err
			}
		}
		if event.Currency != "" {
			if err := event.Currency.Validate(); err != nil {
				_ = s.accounts.MarkBillingWebhookEventProcessed(ctx, event.Provider, event.EventID, "failed", err.Error())
				return false, err
			}
		}
		if err := s.accounts.ApplyBillingEvent(ctx, event); err != nil {
			_ = s.accounts.MarkBillingWebhookEventProcessed(ctx, event.Provider, event.EventID, "failed", err.Error())
			return false, err
		}
	}
	// invoice / payment_method キャッシュ同期はベストエフォート。失敗しても accounts
	// 更新は確定済みなので webhook は ack（重複再送は dedup されるため、取りこぼしは
	// reconcile バックフィルで修復する）。
	s.applyWebhookSideEffects(ctx, event)
	if err := s.accounts.MarkBillingWebhookEventProcessed(ctx, event.Provider, event.EventID, "processed", ""); err != nil {
		return true, err
	}
	return true, nil
}

// applyWebhookSideEffects mirrors invoice / payment_method webhook payloads into the
// local cache tables. Best-effort: errors are logged, not propagated.
func (s *billingService) applyWebhookSideEffects(ctx context.Context, event *domain.ProviderWebhookEvent) {
	if s.usage == nil {
		return
	}
	if event.Invoice != nil {
		if err := s.usage.UpsertInvoice(ctx, event.ExternalCustomerID, event.Invoice); err != nil {
			s.logger.Error("billing.webhook.invoice_upsert_failed",
				"error", err.Error(),
				"event_id", event.EventID,
				"external_customer_id", event.ExternalCustomerID,
				"stripe_invoice_id", event.Invoice.StripeInvoiceID,
			)
		}
	}
	if event.PaymentMethod != nil {
		if err := s.usage.UpsertPaymentMethod(ctx, event.ExternalCustomerID, event.PaymentMethod); err != nil {
			s.logger.Error("billing.webhook.payment_method_upsert_failed",
				"error", err.Error(),
				"event_id", event.EventID,
				"external_customer_id", event.ExternalCustomerID,
				"stripe_payment_method_id", event.PaymentMethod.StripePaymentMethodID,
			)
		}
	}
	if event.PaymentMethodDeleted != "" {
		if err := s.usage.DeletePaymentMethod(ctx, event.PaymentMethodDeleted); err != nil {
			s.logger.Error("billing.webhook.payment_method_delete_failed",
				"error", err.Error(),
				"event_id", event.EventID,
				"stripe_payment_method_id", event.PaymentMethodDeleted,
			)
		}
	}
	if event.DefaultPaymentMethodSet {
		if err := s.usage.SetDefaultPaymentMethod(ctx, event.ExternalCustomerID, event.DefaultPaymentMethod); err != nil {
			s.logger.Error("billing.webhook.default_payment_method_failed",
				"error", err.Error(),
				"event_id", event.EventID,
				"external_customer_id", event.ExternalCustomerID,
				"default_payment_method", event.DefaultPaymentMethod,
			)
		}
	}
}

type BillingReconciliationDiff struct {
	AccountID          string
	LocalPlan          string
	RemotePlan         string
	LocalStatus        string
	RemoteStatus       string
	LocalSubscription  string
	RemoteSubscription string
	LocalPriceID       string
	RemotePriceID      string
}

func (s *billingService) ReconcileAccount(ctx context.Context, accountID string, apply bool) (*BillingReconciliationDiff, error) {
	if s.provider == nil {
		return nil, domain.ErrBillingProviderNotConfigured
	}
	account, err := s.accounts.GetAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return s.reconcileAccount(ctx, account, apply)
}

func (s *billingService) ReconcileLinkedAccounts(ctx context.Context, apply bool, limit int) ([]*BillingReconciliationDiff, error) {
	if s.provider == nil {
		return nil, domain.ErrBillingProviderNotConfigured
	}
	accounts, err := s.accounts.ListStripeLinkedAccounts(ctx, limit)
	if err != nil {
		return nil, err
	}
	diffs := make([]*BillingReconciliationDiff, 0, len(accounts))
	for _, account := range accounts {
		diff, err := s.reconcileAccount(ctx, account, apply)
		if err != nil {
			s.logger.Error("billing.reconciliation.account_failed", "error", err.Error(), "account_id", account.AccountID)
			return diffs, err
		}
		diffs = append(diffs, diff)
	}
	s.logger.Info("billing.reconciliation.completed", "apply", apply, "count", len(diffs))
	return diffs, nil
}

func (s *billingService) reconcileAccount(ctx context.Context, account *domain.Account, apply bool) (*BillingReconciliationDiff, error) {
	remote, err := s.provider.FetchBillingState(ctx, account)
	if err != nil {
		return nil, err
	}
	// apply 時は invoice / payment method キャッシュもバックフィルする（webhook 取りこぼし修復）。
	if apply {
		s.backfillInvoices(ctx, account)
		s.backfillPaymentMethods(ctx, account)
	}
	diff := &BillingReconciliationDiff{
		AccountID:          account.AccountID,
		LocalPlan:          account.Plan,
		RemotePlan:         string(remote.Plan),
		LocalStatus:        account.BillingStatus,
		RemoteStatus:       string(remote.Status),
		LocalSubscription:  account.StripeSubscriptionID,
		RemoteSubscription: remote.ExternalSubscriptionID,
		LocalPriceID:       account.StripePriceID,
		RemotePriceID:      remote.ExternalPriceID,
	}
	if !apply || (diff.LocalPlan == diff.RemotePlan && diff.LocalStatus == diff.RemoteStatus && diff.LocalSubscription == diff.RemoteSubscription && diff.LocalPriceID == diff.RemotePriceID) {
		s.logger.Info("billing.reconciliation.diff",
			"account_id", diff.AccountID,
			"apply", apply,
			"local_plan", diff.LocalPlan,
			"remote_plan", diff.RemotePlan,
			"local_status", diff.LocalStatus,
			"remote_status", diff.RemoteStatus,
			"local_subscription", diff.LocalSubscription,
			"remote_subscription", diff.RemoteSubscription,
			"local_price_id", diff.LocalPriceID,
			"remote_price_id", diff.RemotePriceID,
		)
		return diff, nil
	}
	if err := s.accounts.ApplyBillingEvent(ctx, remote); err != nil {
		return diff, err
	}
	s.logger.Info("billing.reconciliation.applied", "account_id", diff.AccountID)
	return diff, nil
}

// backfillInvoices pulls historical invoices from the provider and upserts them into
// the cache table. Best-effort: errors are logged, not propagated.
func (s *billingService) backfillInvoices(ctx context.Context, account *domain.Account) {
	if s.usage == nil || account == nil || account.StripeCustomerID == "" {
		return
	}
	invoices, err := s.provider.ListRemoteInvoices(ctx, account, 100)
	if err != nil {
		s.logger.Warn("billing.reconciliation.invoices_fetch_failed", "error", err.Error(), "account_id", account.AccountID)
		return
	}
	for _, inv := range invoices {
		if err := s.usage.UpsertInvoice(ctx, account.StripeCustomerID, inv); err != nil {
			s.logger.Warn("billing.reconciliation.invoice_upsert_failed", "error", err.Error(), "account_id", account.AccountID, "stripe_invoice_id", inv.StripeInvoiceID)
		}
	}
}

// backfillPaymentMethods pulls the customer's payment methods + default from the provider
// and upserts them into the cache table. Best-effort: errors are logged, not propagated.
func (s *billingService) backfillPaymentMethods(ctx context.Context, account *domain.Account) {
	if s.usage == nil || account == nil || account.StripeCustomerID == "" {
		return
	}
	pms, defaultPM, err := s.provider.ListRemotePaymentMethods(ctx, account)
	if err != nil {
		s.logger.Warn("billing.reconciliation.payment_methods_fetch_failed", "error", err.Error(), "account_id", account.AccountID)
		return
	}
	for _, pm := range pms {
		if err := s.usage.UpsertPaymentMethod(ctx, account.StripeCustomerID, pm); err != nil {
			s.logger.Warn("billing.reconciliation.payment_method_upsert_failed", "error", err.Error(), "account_id", account.AccountID, "stripe_payment_method_id", pm.StripePaymentMethodID)
		}
	}
	if err := s.usage.SetDefaultPaymentMethod(ctx, account.StripeCustomerID, defaultPM); err != nil {
		s.logger.Warn("billing.reconciliation.default_payment_method_failed", "error", err.Error(), "account_id", account.AccountID)
	}
}

func (s *billingService) authorizeAccount(ctx context.Context, accountID, userID string) (*domain.Account, error) {
	ok, err := s.accounts.IsAccountAccessible(ctx, accountID, userID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrNotFound
	}
	return s.accounts.GetAccount(ctx, accountID)
}

func (s *billingService) noticeError(ctx context.Context, event string, err error, attrs map[string]any) {
	if !shouldNoticeBillingError(err) {
		return
	}
	txn := newrelic.FromContext(ctx)
	if txn == nil || err == nil {
		return
	}
	txn.AddAttribute("event", event)
	for key, value := range attrs {
		txn.AddAttribute(key, value)
	}
	txn.NoticeError(err)
}

func (s *billingService) logAuthorizeError(ctx context.Context, event string, err error, attrs ...any) {
	attrs = append([]any{"error", err.Error()}, attrs...)
	if errors.Is(err, domain.ErrNotFound) {
		s.logger.Warn(event, attrs...)
		return
	}
	s.logger.Error(event, attrs...)
}

func shouldNoticeBillingError(err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, domain.ErrBillingPlanInvalid):
		return false
	case errors.Is(err, domain.ErrBillingCurrencyUnsupported):
		return false
	case errors.Is(err, domain.ErrBillingWebhookSignatureInvalid):
		return false
	case errors.Is(err, domain.ErrNotFound):
		return false
	default:
		return true
	}
}

// =========================================================
// Usage-Based Billing
// 仕様: docs/architecture/usage-based-billing-spec.md
// =========================================================

func (s *billingService) GetUsage(ctx context.Context, accountID, actorUserID string, periodStart, periodEnd string) (*domain.UsageReport, error) {
	if _, err := s.authorizeAccount(ctx, accountID, actorUserID); err != nil {
		s.logAuthorizeError(ctx, "billing.get_usage.authorize_failed", err,
			"account_id", accountID,
			"actor_user_id", actorUserID,
		)
		return nil, err
	}
	if s.usage == nil {
		return &domain.UsageReport{
			AccountID:   accountID,
			PeriodStart: periodStart,
			PeriodEnd:   periodEnd,
			TotalCost:   "0.00",
			Currency:    "usd",
		}, nil
	}
	byModel, currency, err := s.usage.ListUsageByModel(ctx, accountID, periodStart, periodEnd)
	if err != nil {
		return nil, err
	}
	byDay, err := s.usage.ListDailyUsage(ctx, accountID, periodStart, periodEnd, currency)
	if err != nil {
		return nil, err
	}
	totalMinor := int64(0)
	for _, row := range byModel {
		minor, err := parseMinor(row.TotalCost, currency)
		if err != nil {
			continue
		}
		totalMinor += minor
	}
	return &domain.UsageReport{
		AccountID:   accountID,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		TotalCost:   formatMinor(totalMinor, currency),
		Currency:    currency,
		ByModel:     byModel,
		ByDay:       byDay,
	}, nil
}

func (s *billingService) RecordUsage(ctx context.Context, ev *domain.UsageEvent) (*domain.UsageRecordResult, error) {
	if ev == nil || ev.EventID == "" || ev.AccountID == "" || ev.Model == "" {
		return nil, domain.ErrBillingUsageEventInvalid
	}
	// 認可は handler 側で service principal を要求済み (X-Synthify-Service-Token).

	// When the usage repository is not wired (early dev), keep the legacy logging stub
	// so the worker pipeline still flows; still attempt to push to Stripe meter.
	if s.recorder == nil {
		s.logger.Info("billing.record_usage.stub",
			"account_id", ev.AccountID,
			"workspace_id", ev.WorkspaceID,
			"job_id", ev.JobID,
			"model", ev.Model,
			"input_tokens", ev.InputTokens,
			"output_tokens", ev.OutputTokens,
		)
		s.reportStripeMeterPortion(ctx, ev, ev.InputTokens, ev.OutputTokens)
		return &domain.UsageRecordResult{EventID: ev.EventID, Cost: "0.00"}, nil
	}

	pue := toPlatformEvent(ev)
	result, err := s.recorder.Record(ctx, pue)
	if err != nil {
		if errors.Is(err, platformusage.ErrEventInvalid) {
			return nil, domain.ErrBillingUsageEventInvalid
		}
		return nil, err
	}
	// Recorder mutated pue with the chosen split; mirror the relevant fields
	// back onto the caller's domain event so callers reading ev after the
	// fact see the same accounting.
	ev.CostMinor = pue.CostMinor
	ev.Currency = pue.Currency
	ev.PaidVia = domain.PaidVia(pue.PaidVia)
	ev.CreditAmountMinor = pue.CreditAmountMinor
	ev.StripeAmountMinor = pue.StripeAmountMinor

	// Pricing が解決できず cost 0 で記録された場合は課金漏れ。recorder 側で
	// error ログは出るが、ここでも NewRelic に notice してアラート可能にする。
	if result.PricingMissing {
		s.noticeError(ctx, "billing.record_usage.pricing_missing", domain.ErrBillingUsagePricingMissing, map[string]any{
			"account_id":    ev.AccountID,
			"event_id":      ev.EventID,
			"model":         ev.Model,
			"input_tokens":  ev.InputTokens,
			"output_tokens": ev.OutputTokens,
		})
	}

	// Stripe meter event: only the Stripe portion, prorated by cost.
	// The worker (PR 9) writes events via the platform Recorder but does
	// not call Stripe directly; PR 8b will replace this immediate send
	// with a stripe_pending flag + scheduled flush so api min-instances=0
	// no longer means "the moment a worker job costs money, api wakes up".
	if result.StripeAmountMinor > 0 {
		inTok, outTok := platformusage.ProratedTokens(ev.InputTokens, ev.OutputTokens, ev.CostMinor, result.StripeAmountMinor)
		s.reportStripeMeterPortion(ctx, ev, inTok, outTok)
	}

	return &domain.UsageRecordResult{
		EventID:           result.EventID,
		Cost:              result.Cost,
		BudgetExceeded:    result.BudgetExceeded,
		CreditStopped:     result.CreditStopped,
		PaidVia:           domain.PaidVia(result.PaidVia),
		CreditAmountMinor: result.CreditAmountMinor,
		StripeAmountMinor: result.StripeAmountMinor,
		PricingMissing:    result.PricingMissing,
	}, nil
}

func toPlatformEvent(ev *domain.UsageEvent) *platformusage.Event {
	return &platformusage.Event{
		EventID:           ev.EventID,
		AccountID:         ev.AccountID,
		WorkspaceID:       ev.WorkspaceID,
		JobID:             ev.JobID,
		Model:             ev.Model,
		InputTokens:       ev.InputTokens,
		OutputTokens:      ev.OutputTokens,
		CostMinor:         ev.CostMinor,
		Currency:          ev.Currency,
		PaidVia:           platformusage.PaidVia(ev.PaidVia),
		CreditAmountMinor: ev.CreditAmountMinor,
		StripeAmountMinor: ev.StripeAmountMinor,
		CreatedAt:         ev.CreatedAt,
	}
}

func (s *billingService) reportStripeMeterPortion(ctx context.Context, ev *domain.UsageEvent, inputTokens, outputTokens int64) {
	if s.provider == nil {
		return
	}
	account, err := s.accounts.GetAccount(ctx, ev.AccountID)
	if err != nil || account == nil {
		return
	}
	if err := s.provider.ReportTokenUsage(ctx, account, ev.EventID, inputTokens, outputTokens); err != nil {
		s.logger.Warn("billing.record_usage.meter_event_failed",
			"error", err.Error(),
			"account_id", ev.AccountID,
			"event_id", ev.EventID,
		)
	}
}

// formatMinor renders a minor-unit amount as a decimal string in the conventional
// presentation for the currency (2 decimal places for cents currencies, integer for JPY).
func formatMinor(minor int64, currency string) string {
	if currency == string(domain.BillingCurrencyJPY) {
		return strconv.FormatInt(minor, 10)
	}
	neg := ""
	if minor < 0 {
		neg = "-"
		minor = -minor
	}
	return neg + strconv.FormatInt(minor/100, 10) + "." + fmt.Sprintf("%02d", minor%100)
}

func parseMinor(value string, currency string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	if currency == string(domain.BillingCurrencyJPY) {
		return strconv.ParseInt(value, 10, 64)
	}
	neg := false
	if strings.HasPrefix(value, "-") {
		neg = true
		value = strings.TrimPrefix(value, "-")
	}
	whole, frac, ok := strings.Cut(value, ".")
	if !ok {
		frac = ""
	}
	if whole == "" {
		whole = "0"
	}
	if len(frac) > 2 {
		return 0, domain.ErrBillingBudgetInvalid
	}
	for len(frac) < 2 {
		frac += "0"
	}
	wholeMinor, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, err
	}
	fracMinor := int64(0)
	if frac != "" {
		fracMinor, err = strconv.ParseInt(frac, 10, 64)
		if err != nil {
			return 0, err
		}
	}
	minor := wholeMinor*100 + fracMinor
	if neg {
		minor = -minor
	}
	return minor, nil
}

func (s *billingService) UpdateBudget(ctx context.Context, accountID, actorUserID string, budgetLimit string) (string, error) {
	account, err := s.authorizeAccount(ctx, accountID, actorUserID)
	if err != nil {
		s.logAuthorizeError(ctx, "billing.update_budget.authorize_failed", err,
			"account_id", accountID,
			"actor_user_id", actorUserID,
		)
		return "", err
	}
	currency := account.BillingCurrency
	if currency == "" {
		currency = "usd"
	}
	limitMinor, err := parseMinor(budgetLimit, currency)
	if err != nil || limitMinor < 0 {
		return "", domain.ErrBillingBudgetInvalid
	}
	if s.usage == nil {
		return formatMinor(limitMinor, currency), nil
	}
	if err := s.usage.UpdateAccountBudgetLimit(ctx, accountID, limitMinor); err != nil {
		return "", err
	}
	return formatMinor(limitMinor, currency), nil
}

func (s *billingService) ListInvoices(ctx context.Context, accountID, actorUserID string, limit int) (*domain.InvoiceList, error) {
	if _, err := s.authorizeAccount(ctx, accountID, actorUserID); err != nil {
		s.logAuthorizeError(ctx, "billing.list_invoices.authorize_failed", err,
			"account_id", accountID,
			"actor_user_id", actorUserID,
		)
		return nil, err
	}
	if s.usage == nil {
		return &domain.InvoiceList{Invoices: nil, UpcomingAmount: "0.00", UpcomingPeriodEnd: ""}, nil
	}
	return s.usage.ListInvoices(ctx, accountID, limit)
}

func (s *billingService) ListPaymentMethods(ctx context.Context, accountID, actorUserID string) ([]*domain.PaymentMethod, error) {
	if _, err := s.authorizeAccount(ctx, accountID, actorUserID); err != nil {
		s.logAuthorizeError(ctx, "billing.list_payment_methods.authorize_failed", err,
			"account_id", accountID,
			"actor_user_id", actorUserID,
		)
		return nil, err
	}
	if s.usage == nil {
		return nil, nil
	}
	return s.usage.ListPaymentMethods(ctx, accountID)
}

// =========================================================
// Credits
// =========================================================

// GrantFreeSignupCredit は新規アカウント作成時に $1 の無料クレジットを付与する。
// 既に付与済みの場合は冪等に何もしない（credit_id が衝突するため）。
func (s *billingService) GrantFreeSignupCredit(ctx context.Context, accountID string) error {
	if s.usage == nil {
		return nil
	}
	grant := &domain.CreditGrant{
		CreditID:    "free-signup-" + accountID,
		AccountID:   accountID,
		CreditType:  domain.CreditTypeFree,
		AmountMinor: domain.FreeSignupCreditMinor,
		Currency:    "usd",
		Note:        "signup bonus",
		GrantedBy:   "system",
		GrantedAt:   s.now().UTC().Format("2006-01-02T15:04:05Z"),
	}
	if err := s.usage.GrantCredit(ctx, grant); err != nil {
		s.logger.Warn("billing.grant_free_signup_credit.failed",
			"error", err.Error(),
			"account_id", accountID,
		)
		return err
	}
	s.logger.Info("billing.grant_free_signup_credit.ok",
		"account_id", accountID,
		"amount_minor", domain.FreeSignupCreditMinor,
	)
	return nil
}

// GrantCredit は admin が任意アカウントにクレジットを付与する。
func (s *billingService) GrantCredit(ctx context.Context, actorUserID, accountID string, amountMinor int64, note string) (*domain.CreditGrant, error) {
	if amountMinor <= 0 {
		return nil, domain.ErrBillingBudgetInvalid
	}
	if s.usage == nil {
		return nil, domain.ErrBillingProviderNotConfigured
	}
	grant := &domain.CreditGrant{
		CreditID:    newCreditID(),
		AccountID:   accountID,
		CreditType:  domain.CreditTypeFree,
		AmountMinor: amountMinor,
		Currency:    "usd",
		Note:        note,
		GrantedBy:   actorUserID,
		GrantedAt:   s.now().UTC().Format("2006-01-02T15:04:05Z"),
	}
	if err := s.usage.GrantCredit(ctx, grant); err != nil {
		return nil, err
	}
	return grant, nil
}

func (s *billingService) GetCreditBalance(ctx context.Context, accountID, actorUserID string) (int64, error) {
	if _, err := s.authorizeAccount(ctx, accountID, actorUserID); err != nil {
		return 0, err
	}
	if s.usage == nil {
		return 0, nil
	}
	return s.usage.GetCreditBalance(ctx, accountID)
}

func newCreditID() string {
	return fmt.Sprintf("credit-%d", time.Now().UnixNano())
}
