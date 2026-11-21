package bootstrap

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

// An unset key must never match an empty or missing one: readiness fails
// closed (docs/architecture/stage-prod-smoke-tests.md), like the worker's.
func TestHealthHandler_ReadinessFailsClosedWhenAKeyIsUnset(t *testing.T) {
	for _, tc := range []struct {
		name        string
		readiness   string
		monitor     string
		target      string
		header      string
		wantCode    int
		wantChecked bool
	}{
		{name: "both keys unset, no key", target: "/health?ready=1", wantCode: http.StatusUnauthorized},
		{name: "both keys unset, empty query key", target: "/health?ready=1&key=", wantCode: http.StatusUnauthorized},
		{name: "both keys unset, some key", target: "/health?ready=1", header: "anything", wantCode: http.StatusUnauthorized},
		{name: "monitor key unset, no key", readiness: "expected", target: "/health?ready=1", wantCode: http.StatusUnauthorized},
		{name: "monitor key unset, empty query key", readiness: "expected", target: "/health?ready=1&key=", wantCode: http.StatusUnauthorized},
		{name: "readiness key unset, no key", monitor: "monitor-expected", target: "/health?ready=1", wantCode: http.StatusUnauthorized},
		{name: "monitor key unset, readiness key still works", readiness: "expected", target: "/health?ready=1", header: "expected", wantCode: http.StatusOK, wantChecked: true},
		{name: "readiness key unset, monitor key still works", monitor: "monitor-expected", target: "/health?ready=1", header: "monitor-expected", wantCode: http.StatusOK, wantChecked: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &healthChecker{}
			req := httptest.NewRequest(http.MethodGet, tc.target, nil)
			if tc.header != "" {
				req.Header.Set("X-Synthify-Readiness-Key", tc.header)
			}
			rec := httptest.NewRecorder()

			healthHandler(p, tc.readiness, tc.monitor)(rec, req)

			if rec.Code != tc.wantCode {
				t.Fatalf("expected %d, got %d", tc.wantCode, rec.Code)
			}
			if p.called != tc.wantChecked {
				t.Fatalf("dependency check called = %v, want %v", p.called, tc.wantChecked)
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
