package joblog

import (
	"context"
	"log"
)

// NoopLogger does nothing but can mirror to stdout if needed.
type NoopLogger struct {
	MirrorToStdout bool
}

// Log handles the event by conditionally printing it to standard output if MirrorToStdout is set.
func (l *NoopLogger) Log(ctx context.Context, e Event) {
	if l.MirrorToStdout {
		log.Printf("[%s] %s: %s (job=%s, ws=%s, doc=%s) detail=%+v",
			e.Level, e.Event, e.Message, e.JobID, e.WorkspaceID, e.DocumentID, e.Detail)
	}
}
