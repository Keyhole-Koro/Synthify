// Package joblog provides top-level aliases and re-exports of joblog logging abstractions.
package joblog

import (
	"context"

	sharedjoblog "github.com/synthify/backend/apps/monitor/joblog"
)

// Level aliases sharedjoblog.Level.
type Level = sharedjoblog.Level

// INFO aliases sharedjoblog.INFO.
const INFO = sharedjoblog.INFO

// WARN aliases sharedjoblog.WARN.
const WARN = sharedjoblog.WARN

// ERROR aliases sharedjoblog.ERROR.
const ERROR = sharedjoblog.ERROR

// Event aliases sharedjoblog.Event.
type Event = sharedjoblog.Event

// Logger aliases sharedjoblog.Logger.
type Logger = sharedjoblog.Logger

// NoopLogger aliases sharedjoblog.NoopLogger.
type NoopLogger = sharedjoblog.NoopLogger

// WithLogger attaches a logger instance to context, delegating to sharedjoblog.WithLogger.
var WithLogger = func(ctx context.Context, l Logger) context.Context {
	return sharedjoblog.WithLogger(ctx, l)
}
// FromContext retrieves the logger from context, delegating to sharedjoblog.FromContext.
var FromContext = func(ctx context.Context) Logger {
	return sharedjoblog.FromContext(ctx)
}
