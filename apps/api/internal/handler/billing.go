package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	connect "connectrpc.com/connect"
	"github.com/synthify/backend/apps/api/internal/application"
	"github.com/synthify/backend/apps/api/internal/domain"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

type BillingHandler struct {
	service application.BillingUsecase
}

func NewBillingHandler(svc application.BillingUsecase) *BillingHandler {
	return &BillingHandler{service: svc}
}

func NewBillingWebhookHTTPHandler(svc application.BillingUsecase, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			logger.Warn("billing.webhook.read_failed", "error", err.Error(), "path", r.URL.Path)
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		if err := svc.HandleWebhook(r.Context(), payload, r.Header.Get("Stripe-Signature")); err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, domain.ErrBillingWebhookSignatureInvalid) {
				status = http.StatusBadRequest
			}
			http.Error(w, "webhook failed", status)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
}

func (h *BillingHandler) GetBillingAccount(ctx context.Context, req *connect.Request[appv1.GetBillingAccountRequest]) (*connect.Response[appv1.GetBillingAccountResponse], error) {
	if req.Msg.GetAccountId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("account_id is required"))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	account, err := h.application.GetBillingAccount(ctx, req.Msg.GetAccountId(), userID)
	if err != nil {
		return nil, toError(err)
	}
	creditBalance, _ := h.application.GetCreditBalance(ctx, req.Msg.GetAccountId(), userID)
	return connect.NewResponse(&appv1.GetBillingAccountResponse{
		AccountId:              account.AccountID,
		Plan:                   account.Plan,
		BillingStatus:          account.BillingStatus,
		StorageQuotaBytes:      account.StorageQuotaBytes,
		StorageUsedBytes:       account.StorageUsedBytes,
		MaxFileSizeBytes:       account.MaxFileSizeBytes,
		StripePriceId:          account.StripePriceID,
		BillingCurrency:        account.BillingCurrency,
		BillingInterval:        account.BillingInterval,
		BillingAmountMinor:     account.BillingAmountMinor,
		CurrentPeriodEnd:       account.CurrentPeriodEnd,
		CancelAtPeriodEnd:      account.CancelAtPeriodEnd,
		BudgetLimit:            minorToDecimal(account.BudgetLimitMinor),
		CurrentPeriodUsage:     minorToDecimal(account.CurrentPeriodUsageMinor),
		CurrentPeriodStartedAt: account.CurrentPeriodStartedAt,
		BudgetExceeded:         account.BudgetExceeded,
		CreditBalance:          minorToDecimal(creditBalance),
		CreditStopped:          account.Plan == string(domain.BillingPlanFree) && creditBalance <= 0,
	}), nil
}

// minorToDecimal converts cents (or equivalent minor unit) to a decimal string like "12.34".
func minorToDecimal(minor int64) string {
	return fmt.Sprintf("%d.%02d", minor/100, abs64(minor%100))
}

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

func (h *BillingHandler) CreateCheckoutSession(ctx context.Context, req *connect.Request[appv1.CreateCheckoutSessionRequest]) (*connect.Response[appv1.CreateCheckoutSessionResponse], error) {
	if req.Msg.GetAccountId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("account_id is required"))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	session, err := h.application.CreateCheckoutSession(ctx, req.Msg.GetAccountId(), userID, domain.BillingPlanUsageBased, domain.BillingCurrency(req.Msg.GetCurrency()))
	if err != nil {
		return nil, toError(err)
	}
	return connect.NewResponse(&appv1.CreateCheckoutSessionResponse{
		CheckoutUrl: session.URL,
	}), nil
}

func (h *BillingHandler) CreatePortalSession(ctx context.Context, req *connect.Request[appv1.CreatePortalSessionRequest]) (*connect.Response[appv1.CreatePortalSessionResponse], error) {
	if req.Msg.GetAccountId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("account_id is required"))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	session, err := h.application.CreatePortalSession(ctx, req.Msg.GetAccountId(), userID)
	if err != nil {
		return nil, toError(err)
	}
	return connect.NewResponse(&appv1.CreatePortalSessionResponse{
		PortalUrl: session.URL,
	}), nil
}

// =========================================================
// Usage-Based Billing handlers
// =========================================================

func (h *BillingHandler) GetUsage(ctx context.Context, req *connect.Request[appv1.GetUsageRequest]) (*connect.Response[appv1.GetUsageResponse], error) {
	if req.Msg.GetAccountId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("account_id is required"))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	report, err := h.application.GetUsage(ctx, req.Msg.GetAccountId(), userID, req.Msg.GetPeriodStart(), req.Msg.GetPeriodEnd())
	if err != nil {
		return nil, toError(err)
	}
	resp := &appv1.GetUsageResponse{
		AccountId:   report.AccountID,
		PeriodStart: report.PeriodStart,
		PeriodEnd:   report.PeriodEnd,
		TotalCost:   report.TotalCost,
		Currency:    report.Currency,
	}
	for _, m := range report.ByModel {
		resp.ByModel = append(resp.ByModel, &appv1.ModelUsage{
			Model:        m.Model,
			InputTokens:  m.InputTokens,
			OutputTokens: m.OutputTokens,
			InputCost:    m.InputCost,
			OutputCost:   m.OutputCost,
			TotalCost:    m.TotalCost,
			EventCount:   m.EventCount,
		})
	}
	for _, d := range report.ByDay {
		resp.ByDay = append(resp.ByDay, &appv1.DailyUsage{
			Date:      d.Date,
			TotalCost: d.TotalCost,
		})
	}
	return connect.NewResponse(resp), nil
}

func (h *BillingHandler) RecordUsage(ctx context.Context, req *connect.Request[appv1.RecordUsageRequest]) (*connect.Response[appv1.RecordUsageResponse], error) {
	// 内部 RPC: worker -> API のサービストークン認証のみ受け付ける。
	// SYNTHIFY_INTERNAL_SERVICE_TOKEN が未設定の環境では service principal が立たないので、
	// すべての RecordUsage は拒否される。
	if _, err := requireServicePrincipal(ctx); err != nil {
		return nil, err
	}
	if req.Msg.GetAccountId() == "" || req.Msg.GetModel() == "" || req.Msg.GetEventId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("event_id, account_id, and model are required"))
	}
	ev := &domain.UsageEvent{
		EventID:      req.Msg.GetEventId(),
		AccountID:    req.Msg.GetAccountId(),
		WorkspaceID:  req.Msg.GetWorkspaceId(),
		JobID:        req.Msg.GetJobId(),
		Model:        req.Msg.GetModel(),
		InputTokens:  req.Msg.GetInputTokens(),
		OutputTokens: req.Msg.GetOutputTokens(),
	}
	result, err := h.application.RecordUsage(ctx, ev)
	if err != nil {
		return nil, toError(err)
	}
	return connect.NewResponse(&appv1.RecordUsageResponse{
		EventId:        result.EventID,
		Cost:           result.Cost,
		BudgetExceeded: result.BudgetExceeded,
	}), nil
}

func (h *BillingHandler) UpdateBudget(ctx context.Context, req *connect.Request[appv1.UpdateBudgetRequest]) (*connect.Response[appv1.UpdateBudgetResponse], error) {
	if req.Msg.GetAccountId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("account_id is required"))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	limit, err := h.application.UpdateBudget(ctx, req.Msg.GetAccountId(), userID, req.Msg.GetBudgetLimit())
	if err != nil {
		return nil, toError(err)
	}
	return connect.NewResponse(&appv1.UpdateBudgetResponse{BudgetLimit: limit}), nil
}

func (h *BillingHandler) ListInvoices(ctx context.Context, req *connect.Request[appv1.ListInvoicesRequest]) (*connect.Response[appv1.ListInvoicesResponse], error) {
	if req.Msg.GetAccountId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("account_id is required"))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	list, err := h.application.ListInvoices(ctx, req.Msg.GetAccountId(), userID, int(req.Msg.GetLimit()))
	if err != nil {
		return nil, toError(err)
	}
	resp := &appv1.ListInvoicesResponse{
		UpcomingAmount:    list.UpcomingAmount,
		UpcomingPeriodEnd: list.UpcomingPeriodEnd,
	}
	for _, inv := range list.Invoices {
		resp.Invoices = append(resp.Invoices, &appv1.Invoice{
			InvoiceId:        inv.InvoiceID,
			Amount:           inv.Amount,
			Currency:         inv.Currency,
			Status:           inv.Status,
			HostedInvoiceUrl: inv.HostedInvoiceURL,
			InvoicePdfUrl:    inv.InvoicePDFURL,
			PeriodStart:      inv.PeriodStart,
			PeriodEnd:        inv.PeriodEnd,
			PaidAt:           inv.PaidAt,
			CreatedAt:        inv.CreatedAt,
		})
	}
	return connect.NewResponse(resp), nil
}

func (h *BillingHandler) GrantCredit(ctx context.Context, req *connect.Request[appv1.GrantCreditRequest]) (*connect.Response[appv1.GrantCreditResponse], error) {
	if req.Msg.GetAccountId() == "" || req.Msg.GetAmountMinor() <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("account_id and amount_minor > 0 are required"))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := requireAdminPrincipal(ctx); err != nil {
		return nil, err
	}
	grant, err := h.application.GrantCredit(ctx, userID, req.Msg.GetAccountId(), req.Msg.GetAmountMinor(), req.Msg.GetNote())
	if err != nil {
		return nil, toError(err)
	}
	balance, _ := h.application.GetCreditBalance(ctx, req.Msg.GetAccountId(), userID)
	return connect.NewResponse(&appv1.GrantCreditResponse{
		CreditId:      grant.CreditID,
		AccountId:     grant.AccountID,
		AmountMinor:   grant.AmountMinor,
		CreditBalance: minorToDecimal(balance),
	}), nil
}

func (h *BillingHandler) GetCreditBalance(ctx context.Context, req *connect.Request[appv1.GetCreditBalanceRequest]) (*connect.Response[appv1.GetCreditBalanceResponse], error) {
	if req.Msg.GetAccountId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("account_id is required"))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	balance, err := h.application.GetCreditBalance(ctx, req.Msg.GetAccountId(), userID)
	if err != nil {
		return nil, toError(err)
	}
	account, _ := h.application.GetBillingAccount(ctx, req.Msg.GetAccountId(), userID)
	stopped := account != nil && account.Plan == string(domain.BillingPlanFree) && balance <= 0
	return connect.NewResponse(&appv1.GetCreditBalanceResponse{
		CreditBalance: minorToDecimal(balance),
		CreditStopped: stopped,
	}), nil
}

func (h *BillingHandler) ListPaymentMethods(ctx context.Context, req *connect.Request[appv1.ListPaymentMethodsRequest]) (*connect.Response[appv1.ListPaymentMethodsResponse], error) {
	if req.Msg.GetAccountId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("account_id is required"))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	methods, err := h.application.ListPaymentMethods(ctx, req.Msg.GetAccountId(), userID)
	if err != nil {
		return nil, toError(err)
	}
	resp := &appv1.ListPaymentMethodsResponse{}
	for _, pm := range methods {
		resp.PaymentMethods = append(resp.PaymentMethods, &appv1.PaymentMethod{
			PaymentMethodId: pm.PaymentMethodID,
			Brand:           pm.Brand,
			Last4:           pm.Last4,
			ExpMonth:        pm.ExpMonth,
			ExpYear:         pm.ExpYear,
			IsDefault:       pm.IsDefault,
		})
	}
	return connect.NewResponse(resp), nil
}
