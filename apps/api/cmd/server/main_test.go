package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
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

	healthHandler(p, "", "")(rec, req)

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

			healthHandler(p, "expected", "monitor-expected")(rec, req)

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

	healthHandler(p, "expected", "monitor-expected")(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !p.called {
		t.Fatal("authorized readiness request should call dependency checks")
	}
}

func TestHealthHandler_ReadinessChecksDependenciesWithValidMonitorKey(t *testing.T) {
	p := &healthChecker{}
	req := httptest.NewRequest(http.MethodGet, "/health?ready=1", nil)
	req.Header.Set("X-Synthify-Readiness-Key", "monitor-expected")
	rec := httptest.NewRecorder()

	healthHandler(p, "expected", "monitor-expected")(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !p.called {
		t.Fatal("authorized readiness request (monitor key) should call dependency checks")
	}
}

func TestHealthHandler_ReadinessFailsOnDependencyError(t *testing.T) {
	p := &healthChecker{err: errors.New("db down")}
	req := httptest.NewRequest(http.MethodGet, "/health?ready=1", nil)
	req.Header.Set("X-Synthify-Readiness-Key", "expected")
	rec := httptest.NewRecorder()

	healthHandler(p, "expected", "monitor-expected")(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}
