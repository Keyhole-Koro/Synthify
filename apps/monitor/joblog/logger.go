// Package joblog defines structured logging abstractions, event definitions,
// and context helpers for tracking job executions and error reports.
package joblog

import "context"

// Level represents the severity level of a job log event.
type Level string

const (
	// INFO indicates informational events during normal job execution.
	INFO  Level = "INFO"
	// WARN indicates non-fatal warning conditions encountered during a job.
	WARN  Level = "WARN"
	// ERROR indicates an error condition or failure during a job.
	ERROR Level = "ERROR"
)

// Event describes a structured log entry emitted during a background job.
type Event struct {
	JobID       string
	WorkspaceID string
	DocumentID  string
	Level       Level
	Event       string
	Message     string
	Detail      map[string]any // optional
}
// Logger abstracts recording structured job execution events.
type Logger interface {
	Log(ctx context.Context, e Event)
}

type ctxKey struct{}

// WithLogger returns a context with the logger attached.
func WithLogger(ctx context.Context, l Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext returns the logger from the context, or a NoopLogger if not found.
func FromContext(ctx context.Context) Logger {
	if l, ok := ctx.Value(ctxKey{}).(Logger); ok {
		return l
	}
	return &NoopLogger{}
}
