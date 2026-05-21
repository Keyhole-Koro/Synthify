package joblog

import (
	"context"
	"log"
)

// NoopLogger does nothing but can mirror to stdout if needed.
type NoopLogger struct {
	MirrorToStdout bool
}

func (l *NoopLogger) Log(ctx context.Context, e Event) {
	if l.MirrorToStdout {
		log.Printf("[%s] %s: %s (job=%s, ws=%s, doc=%s) detail=%+v",
			e.Level, e.Event, e.Message, e.JobID, e.WorkspaceID, e.DocumentID, e.Detail)
	}
}
