package handler

import (
	"context"
	"errors"
	"log/slog"

	connect "connectrpc.com/connect"
)

// MaskInternalErrors は handler から返った *connect.Error のうち、
// クライアントに本文を晒すと内部情報（DB スキーマ、SQL 文、スタックトレース等）が
// 漏れるコード（Internal / Unknown / DataLoss）について、メッセージを汎用文に差し替えつつ
// 元のエラーをサーバログに残す Connect インターセプター。
//
// それ以外のコード（NotFound / InvalidArgument / PermissionDenied 等）は
// toError 経由で domain エラーから組み立てた安全なメッセージなので素通しする。
func MaskInternalErrors(logger *slog.Logger) connect.UnaryInterceptorFunc {
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
