package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	apiauth "github.com/synthify/backend/apps/api/internal/auth"
)

type fakeSweeper struct {
	resumed     int
	stuck       int
	resumeErr   error
	stuckErr    error
	resumeCalls int
	stuckCalls  int
}

func (f *fakeSweeper) AutoResumeFailedJobs(context.Context) (int, error) {
	f.resumeCalls++
	return f.resumed, f.resumeErr
}

func (f *fakeSweeper) ReportStuckJobs(context.Context) (int, error) {
	f.stuckCalls++
	return f.stuck, f.stuckErr
}

type fakeMaintenanceAuth struct {
	caller string
	err    error
}

func (f fakeMaintenanceAuth) Authenticate(context.Context, string) (string, error) {
	return f.caller, f.err
}

func newMaintenanceHandler(sweeper MaintenanceSweeper, auth MaintenanceAuthenticator) *MaintenanceHTTPHandler {
	return NewMaintenanceHTTPHandler(sweeper, auth, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestMaintenanceHandler_RunsBothSweepsAndReportsCounts(t *testing.T) {
	sweeper := &fakeSweeper{resumed: 2, stuck: 3}
	rec := httptest.NewRecorder()

	newMaintenanceHandler(sweeper, fakeMaintenanceAuth{caller: "sched@example.iam.gserviceaccount.com"}).
		ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/internal/maintenance/sweep", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if sweeper.resumeCalls != 1 || sweeper.stuckCalls != 1 {
		t.Fatalf("expected both sweeps to run once, got resume=%d stuck=%d", sweeper.resumeCalls, sweeper.stuckCalls)
	}
	var body maintenanceSweepResult
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Resumed != 2 || body.Stuck != 3 {
		t.Fatalf("expected the sweep counts in the body, got %+v", body)
	}
}

func TestMaintenanceHandler_RejectsUnauthenticatedBeforeSweeping(t *testing.T) {
	for _, tc := range []struct {
		name     string
		authErr  error
		wantCode int
	}{
		{name: "unauthenticated", authErr: fmt.Errorf("%w: no token", apiauth.ErrUnauthenticated), wantCode: http.StatusUnauthorized},
		{name: "not allowlisted", authErr: fmt.Errorf("%w: other caller", apiauth.ErrPermissionDenied), wantCode: http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sweeper := &fakeSweeper{}
			rec := httptest.NewRecorder()

			newMaintenanceHandler(sweeper, fakeMaintenanceAuth{err: tc.authErr}).
				ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/internal/maintenance/sweep", nil))

			if rec.Code != tc.wantCode {
				t.Fatalf("expected %d, got %d", tc.wantCode, rec.Code)
			}
			if sweeper.resumeCalls != 0 || sweeper.stuckCalls != 0 {
				t.Fatal("a rejected request must not touch the database")
			}
		})
	}
}

func TestMaintenanceHandler_RejectsNonPost(t *testing.T) {
	sweeper := &fakeSweeper{}
	rec := httptest.NewRecorder()

	newMaintenanceHandler(sweeper, fakeMaintenanceAuth{caller: "sched@example.iam.gserviceaccount.com"}).
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/internal/maintenance/sweep", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
	if sweeper.resumeCalls != 0 {
		t.Fatal("GET must not run the sweep")
	}
}

func TestMaintenanceHandler_ReportsSweepFailures(t *testing.T) {
	t.Run("auto resume failure short-circuits the stuck sweep", func(t *testing.T) {
		sweeper := &fakeSweeper{resumeErr: errors.New("db down")}
		rec := httptest.NewRecorder()

		newMaintenanceHandler(sweeper, fakeMaintenanceAuth{caller: "sched@example.iam.gserviceaccount.com"}).
			ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/internal/maintenance/sweep", nil))

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 so Cloud Scheduler records the failure, got %d", rec.Code)
		}
		if sweeper.stuckCalls != 0 {
			t.Fatal("expected the stuck sweep to be skipped after the first failure")
		}
	})

	t.Run("stuck sweep failure", func(t *testing.T) {
		sweeper := &fakeSweeper{stuckErr: errors.New("db down")}
		rec := httptest.NewRecorder()

		newMaintenanceHandler(sweeper, fakeMaintenanceAuth{caller: "sched@example.iam.gserviceaccount.com"}).
			ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/internal/maintenance/sweep", nil))

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 so Cloud Scheduler records the failure, got %d", rec.Code)
		}
	})
}
