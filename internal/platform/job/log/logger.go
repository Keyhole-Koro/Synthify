// Package joblog provides structured logging and event collection for asynchronous jobs.
package joblog

import "context"

// Level represents the severity level of a job log event.
type Level string

const (
	// INFO indicates informational messages during normal job progression.
	INFO Level = "INFO"
	// WARN indicates warnings or non-fatal anomalies during job execution.
	WARN Level = "WARN"
	// ERROR indicates fatal errors or failure conditions during job execution.
	ERROR Level = "ERROR"
)

// Event describes a single structured lifecycle or diagnostic event emitted during job execution.
type Event struct {
	JobID       string
	WorkspaceID string
	DocumentID  string
	Level       Level
	Event       string
	Message     string
	Detail      map[string]any // optional metadata
}

// Logger defines the interface for recording job execution events.
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
