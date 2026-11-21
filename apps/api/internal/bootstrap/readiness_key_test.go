package bootstrap

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The readiness key is read from the X-Synthify-Readiness-Key header only. A
// key in the query string ends up in the Cloud Run request log (and any proxy
// log), so it is refused even when it is right, as the worker does.
func TestHealthHandler_ReadinessIgnoresAKeyInTheQuery(t *testing.T) {
	for _, key := range []string{"expected", "monitor-expected"} {
		t.Run(key, func(t *testing.T) {
			p := &healthChecker{}
			req := httptest.NewRequest(http.MethodGet, "/health?ready=1&key="+key, nil)
			rec := httptest.NewRecorder()

			healthHandler(p, "expected", "monitor-expected")(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("a right key in the query: expected 401, got %d", rec.Code)
			}
			if p.called {
				t.Fatal("a key in the query must not reach the dependency check")
			}

			p = &healthChecker{}
			req = httptest.NewRequest(http.MethodGet, "/health?ready=1", nil)
			req.Header.Set("X-Synthify-Readiness-Key", key)
			rec = httptest.NewRecorder()

			healthHandler(p, "expected", "monitor-expected")(rec, req)

			if rec.Code != http.StatusOK || !p.called {
				t.Fatalf("the same key in the header: got %d (checked %v), want 200 after the dependency check", rec.Code, p.called)
			}
		})
	}
}
