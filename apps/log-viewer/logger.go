package joblog

import (
	"context"

	sharedjoblog "github.com/synthify/backend/packages/shared/job/log"
)

type Level = sharedjoblog.Level

const INFO = sharedjoblog.INFO
const WARN = sharedjoblog.WARN
const ERROR = sharedjoblog.ERROR

type Event = sharedjoblog.Event

type Logger = sharedjoblog.Logger

type NoopLogger = sharedjoblog.NoopLogger

var WithLogger = func(ctx context.Context, l Logger) context.Context {
	return sharedjoblog.WithLogger(ctx, l)
}

var FromContext = func(ctx context.Context) Logger {
	return sharedjoblog.FromContext(ctx)
}
