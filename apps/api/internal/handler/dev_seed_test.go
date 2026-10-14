package handler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/synthify/backend/apps/api/internal/application"
	"github.com/synthify/backend/apps/api/internal/auth"
	"github.com/synthify/backend/apps/api/internal/domain"
)

type devSeedUsecaseStub struct {
	result *application.DevSeedResult
	err    error
	userID string
}

func (s *devSeedUsecaseStub) SeedWorkspace(_ context.Context, userID string) (*application.DevSeedResult, error) {
	s.userID = userID
	return s.result, s.err
}

func devSeedLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func devSeedRequest(method string) *http.Request {
	ctx := auth.ContextWithPrincipal(context.Background(), auth.Principal{
		Kind:      auth.PrincipalKindUser,
		SubjectID: "user_1",
	})
	return httptest.NewRequestWithContext(ctx, method, "/dev/seed-workspace", strings.NewReader("{}"))
}

func TestDevSeedHandlerRejectsWhenDisabled(t *testing.T) {
	h := NewDevSeedHTTPHandler(&devSeedUsecaseStub{}, false, devSeedLogger())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, devSeedRequest(http.MethodPost))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestDevSeedHandlerRequiresPostAndAuthentication(t *testing.T) {
	stub := &devSeedUsecaseStub{}
	h := NewDevSeedHTTPHandler(stub, true, devSeedLogger())

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, devSeedRequest(http.MethodGet))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d, want 405", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/dev/seed-workspace", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want 401", rec.Code)
	}
}

func TestDevSeedHandlerMasksServiceErrors(t *testing.T) {
	h := NewDevSeedHTTPHandler(&devSeedUsecaseStub{err: errors.New("database password")}, true, devSeedLogger())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, devSeedRequest(http.MethodPost))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "database password") {
		t.Fatalf("service error leaked: %q", rec.Body.String())
	}
}

func TestDevSeedHandlerReturnsJSONOnSuccess(t *testing.T) {
	stub := &devSeedUsecaseStub{result: &application.DevSeedResult{
		Workspace: &domain.Workspace{WorkspaceID: "ws_1", Name: "Seed"},
	}}
	h := NewDevSeedHTTPHandler(stub, true, devSeedLogger())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, devSeedRequest(http.MethodPost))
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("response = status %d, content-type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if stub.userID != "user_1" || !strings.Contains(rec.Body.String(), `"workspace_id":"ws_1"`) {
		t.Fatalf("user or body = %q, user=%q", rec.Body.String(), stub.userID)
	}
}
