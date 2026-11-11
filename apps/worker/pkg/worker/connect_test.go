package worker

import (
	"context"
	"errors"
	"testing"

	connect "connectrpc.com/connect"
	"github.com/synthify/backend/internal/domain"
)

func TestWorkerConnectError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code connect.Code
	}{
		{name: "canceled", err: context.Canceled, code: connect.CodeCanceled},
		{name: "deadline exceeded", err: context.DeadlineExceeded, code: connect.CodeDeadlineExceeded},
		{name: "not found", err: domain.ErrNotFound, code: connect.CodeNotFound},
		{name: "forbidden", err: domain.ErrForbidden, code: connect.CodePermissionDenied},
		{name: "conflict", err: domain.ErrConflict, code: connect.CodeAlreadyExists},
		{name: "not implemented", err: domain.ErrNotImplemented, code: connect.CodeUnimplemented},
		{name: "unknown", err: errors.New("provider failed"), code: connect.CodeInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := connect.CodeOf(workerConnectError(tt.err)); got != tt.code {
				t.Fatalf("code = %s, want %s", got, tt.code)
			}
		})
	}
}
