package observability

import (
	"context"
	"errors"
	"log/slog"

	connect "connectrpc.com/connect"
)

// MaskInternalErrorsHandlerOptions returns Connect handler options that:
//   - log every Internal / Unknown / DataLoss (or non-connect) error to slog
//     under the "rpc.internal_error" event, so the failing service's stdout
//     (and therefore Cloud Logging) records the cause without depending on
//     the caller to log it.
//   - replace the response body on those codes with a generic "internal server
//     error" message, so internal details (SQL, stack traces, third-party API
//     payloads) do not leak to clients.
//
// Other codes (NotFound, InvalidArgument, PermissionDenied, ...) come from
// domain mapping and are safe to surface verbatim, so they pass through
// unchanged.
//
// This complements the New Relic interceptor (from ConnectHandlerOptions):
// New Relic captures errors in its Errors inbox via the response, but stdout
// alone would only show the HTTP middleware's "status=500" line. Installing
// both closes the Cloud-Logging gap. Order does not matter.
func MaskInternalErrorsHandlerOptions(logger *slog.Logger) []connect.HandlerOption {
	if logger == nil {
		return nil
	}
	return []connect.HandlerOption{connect.WithInterceptors(maskInternalErrors(logger))}
}

func maskInternalErrors(logger *slog.Logger) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			res, err := next(ctx, req)
			if err == nil {
				return res, nil
			}
			procedure := ""
			if req != nil {
				procedure = req.Spec().Procedure
			}
			var ce *connect.Error
			if !errors.As(err, &ce) {
				logger.Error("rpc.internal_error",
					"error", err.Error(),
					"procedure", procedure,
				)
				return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
			}
			switch ce.Code() {
			case connect.CodeInternal, connect.CodeUnknown, connect.CodeDataLoss:
				logger.Error("rpc.internal_error",
					"error", err.Error(),
					"procedure", procedure,
					"code", ce.Code().String(),
				)
				return nil, connect.NewError(ce.Code(), errors.New("internal server error"))
			}
			return nil, err
		}
	}
}
