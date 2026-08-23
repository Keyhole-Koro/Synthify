package handler

import (
	"context"
	"errors"

	connect "connectrpc.com/connect"
	"github.com/synthify/backend/apps/api/internal/domain"
)

func toError(err error) error {
	if err == nil {
		return nil
	}
	var connectErr *connect.Error
	if errors.As(err, &connectErr) {
		return connectErr
	}

	switch {
	case errors.Is(err, domain.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, domain.ErrForbidden):
		return connect.NewError(connect.CodePermissionDenied, err)
	case errors.Is(err, domain.ErrInvalidArgument):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, domain.ErrConflict):
		return connect.NewError(connect.CodeAlreadyExists, err)
	case errors.Is(err, domain.ErrInvariantViolation):
		// Internal に落とすが、wrapped error の元メッセージはサーバーログに残し、
		// クライアントには汎用 Internal だけ見せる (handler 側のレスポンスマスキング前提)。
		return connect.NewError(connect.CodeInternal, err)
	case errors.Is(err, domain.ErrBillingPlanInvalid):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, domain.ErrBillingCurrencyUnsupported):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, domain.ErrBillingUsageEventInvalid):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, domain.ErrBillingBudgetInvalid):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, domain.ErrBillingBudgetExceeded):
		return connect.NewError(connect.CodeResourceExhausted, err)
	case errors.Is(err, domain.ErrBillingWebhookSignatureInvalid):
		return connect.NewError(connect.CodeUnauthenticated, err)
	case errors.Is(err, domain.ErrUnsupportedDocumentType):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, domain.ErrFileTooLarge), errors.Is(err, domain.ErrStorageQuotaExceeded):
		return connect.NewError(connect.CodeResourceExhausted, err)
	case errors.Is(err, domain.ErrChatSourceUnavailable):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, domain.ErrUploadNotConfirmed), errors.Is(err, domain.ErrUploadSizeMismatch):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, domain.ErrBillingProviderMisconfigured):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, domain.ErrBillingProviderNotConfigured):
		return connect.NewError(connect.CodeUnimplemented, err)
	case errors.Is(err, domain.ErrNotImplemented):
		return connect.NewError(connect.CodeUnimplemented, err)
	case errors.Is(err, domain.ErrApprovalRequired), errors.Is(err, domain.ErrPlanRejected):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, context.Canceled):
		return connect.NewError(connect.CodeCanceled, err)
	case errors.Is(err, context.DeadlineExceeded):
		return connect.NewError(connect.CodeDeadlineExceeded, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}
