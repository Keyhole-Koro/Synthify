package connectutil

import (
	"context"
	"errors"

	connect "connectrpc.com/connect"
	"github.com/synthify/backend/packages/shared/domain"
)

func ToError(err error) error {
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
	case errors.Is(err, domain.ErrBillingPlanInvalid):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, domain.ErrBillingCurrencyUnsupported):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, domain.ErrBillingWebhookSignatureInvalid):
		return connect.NewError(connect.CodeUnauthenticated, err)
	case errors.Is(err, domain.ErrFileTooLarge), errors.Is(err, domain.ErrStorageQuotaExceeded):
		return connect.NewError(connect.CodeResourceExhausted, err)
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
