package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/synthify/backend/apps/worker/pkg/worker/config"
	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
	"github.com/synthify/backend/apps/worker/pkg/worker/llm"
	"github.com/synthify/backend/apps/worker/pkg/worker/metering"
)

type healthChecker struct {
	called bool
	err    error
}

func (c *healthChecker) CheckReadiness(context.Context) error {
	c.called = true
	return c.err
}

func TestHealthHandler_LivenessDoesNotRequireReadinessKey(t *testing.T) {
	p := &healthChecker{}
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	healthHandler(p, "")(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if p.called {
		t.Fatal("liveness should not call dependency checks")
	}
}

func TestHealthHandler_ReadinessRequiresKeyBeforeDependencyChecks(t *testing.T) {
	for _, tc := range []struct {
		name   string
		header string
	}{
		{name: "missing key"},
		{name: "wrong key", header: "wrong"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &healthChecker{}
			req := httptest.NewRequest(http.MethodGet, "/health?ready=1", nil)
			if tc.header != "" {
				req.Header.Set("X-Synthify-Readiness-Key", tc.header)
			}
			rec := httptest.NewRecorder()

			healthHandler(p, "expected")(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d", rec.Code)
			}
			if p.called {
				t.Fatal("unauthorized readiness request should not call dependency checks")
			}
		})
	}
}

func TestHealthHandler_ReadinessChecksDependenciesWithValidKey(t *testing.T) {
	p := &healthChecker{}
	req := httptest.NewRequest(http.MethodGet, "/health?ready=1", nil)
	req.Header.Set("X-Synthify-Readiness-Key", "expected")
	rec := httptest.NewRecorder()

	healthHandler(p, "expected")(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !p.called {
		t.Fatal("authorized readiness request should call dependency checks")
	}
}

func TestHealthHandler_ReadinessFailsOnDependencyError(t *testing.T) {
	p := &healthChecker{err: errors.New("db down")}
	req := httptest.NewRequest(http.MethodGet, "/health?ready=1", nil)
	req.Header.Set("X-Synthify-Readiness-Key", "expected")
	rec := httptest.NewRecorder()

	healthHandler(p, "expected")(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

type processClientBillingTestClient struct{}

func (processClientBillingTestClient) GenerateStructured(context.Context, llm.StructuredRequest) (json.RawMessage, llm.Usage, error) {
	return json.RawMessage(`{}`), llm.Usage{Model: "test", InputTokens: 2, OutputTokens: 1}, nil
}

func (processClientBillingTestClient) GenerateText(context.Context, llm.TextRequest) (string, llm.Usage, error) {
	return "text", llm.Usage{Model: "test", InputTokens: 2, OutputTokens: 1}, nil
}

type processClientBillingTestReporter struct {
	calls int
}

func (r *processClientBillingTestReporter) RecordUsage(context.Context, domain.UsageEvent) error {
	r.calls++
	return nil
}

func TestProcessClientForBillingSkipsLocalProviderUsage(t *testing.T) {
	reporter := &processClientBillingTestReporter{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := processClientForBilling(config.LLM{Provider: config.LLMProviderAntigravity}, processClientBillingTestClient{}, reporter, logger)
	ctx := metering.WithTag(context.Background(), metering.Tag{AccountID: "account", WorkspaceID: "workspace", JobID: "job"})

	_, _, err := client.GenerateText(ctx, llm.TextRequest{})
	if err != nil {
		t.Fatalf("GenerateText() error = %v", err)
	}
	if reporter.calls != 0 {
		t.Fatalf("local provider billing calls = %d, want 0", reporter.calls)
	}
}

func TestProcessClientForBillingReportsGeminiUsage(t *testing.T) {
	reporter := &processClientBillingTestReporter{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := processClientForBilling(config.LLM{Provider: config.LLMProviderGemini}, processClientBillingTestClient{}, reporter, logger)
	ctx := metering.WithTag(context.Background(), metering.Tag{AccountID: "account", WorkspaceID: "workspace", JobID: "job"})

	_, _, err := client.GenerateText(ctx, llm.TextRequest{})
	if err != nil {
		t.Fatalf("GenerateText() error = %v", err)
	}
	if reporter.calls != 1 {
		t.Fatalf("Gemini billing calls = %d, want 1", reporter.calls)
	}
}
