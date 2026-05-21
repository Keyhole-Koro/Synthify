package joblog

import "context"

type Level string

const (
	INFO  Level = "INFO"
	WARN  Level = "WARN"
	ERROR Level = "ERROR"
)

type Event struct {
	JobID       string
	WorkspaceID string
	DocumentID  string
	Level       Level
	Event       string
	Message     string
	Detail      map[string]any // optional
}

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
