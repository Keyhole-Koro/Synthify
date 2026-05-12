package applog

import "context"

type Logger interface {
	Info(ctx context.Context, event string, fields map[string]any)
	Warn(ctx context.Context, event string, err error, fields map[string]any)
	Error(ctx context.Context, event string, err error, fields map[string]any)
}
