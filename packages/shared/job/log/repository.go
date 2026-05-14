package joblog

import "context"

// Repository persists job log events for later retrieval by job/document/workspace scope.
type Repository interface {
	LogJobEvent(ctx context.Context, e Event) error
}
