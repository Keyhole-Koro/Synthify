package joblog

import (
	"context"
	"log"
)

// NoopLogger provides a fallback Logger implementation that discards events or mirrors them to stdout.
type NoopLogger struct {
	MirrorToStdout bool
}
// Log handles the event by printing to standard log if MirrorToStdout is enabled.
func (l *NoopLogger) Log(ctx context.Context, e Event) {
	if l.MirrorToStdout {
		log.Printf("[%s] %s: %s (job=%s, ws=%s, doc=%s) detail=%+v",
			e.Level, e.Event, e.Message, e.JobID, e.WorkspaceID, e.DocumentID, e.Detail)
	}
}
