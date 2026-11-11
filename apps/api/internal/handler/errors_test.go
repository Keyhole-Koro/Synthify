package handler

import (
	"context"
	"errors"
	"fmt"
	"testing"

	connect "connectrpc.com/connect"
	"github.com/synthify/backend/internal/domain"
)

func TestToErrorMapsDomainErrorsAtTransportBoundary(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code connect.Code
	}{
		{name: "not found", err: domain.ErrNotFound, code: connect.CodeNotFound},
		{name: "forbidden", err: domain.ErrForbidden, code: connect.CodePermissionDenied},
		{name: "invalid argument", err: domain.ErrInvalidArgument, code: connect.CodeInvalidArgument},
		{name: "conflict", err: domain.ErrConflict, code: connect.CodeAlreadyExists},
		{name: "quota", err: domain.ErrStorageQuotaExceeded, code: connect.CodeResourceExhausted},
		{name: "upload precondition", err: domain.ErrUploadNotConfirmed, code: connect.CodeFailedPrecondition},
		{name: "unimplemented", err: domain.ErrNotImplemented, code: connect.CodeUnimplemented},
		{name: "canceled", err: context.Canceled, code: connect.CodeCanceled},
		{name: "deadline", err: context.DeadlineExceeded, code: connect.CodeDeadlineExceeded},
		{name: "unknown", err: errors.New("database password"), code: connect.CodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := connect.CodeOf(toError(fmt.Errorf("wrapped: %w", tt.err))); got != tt.code {
				t.Fatalf("code = %s, want %s", got, tt.code)
			}
		})
	}
}

func TestToErrorPreservesExistingConnectError(t *testing.T) {
	want := connect.NewError(connect.CodePermissionDenied, errors.New("denied"))
	if got := toError(want); got != want {
		t.Fatalf("toError() returned a different error: %v", got)
	}
}
