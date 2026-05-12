package applog

import "context"

type NoopLogger struct{}

func (NoopLogger) Info(ctx context.Context, event string, fields map[string]any) {}

func (NoopLogger) Warn(ctx context.Context, event string, err error, fields map[string]any) {}

func (NoopLogger) Error(ctx context.Context, event string, err error, fields map[string]any) {}
