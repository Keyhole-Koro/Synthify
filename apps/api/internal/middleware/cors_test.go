package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testWebOrigin = "https://app.example.com"

// preflight builds the request a browser sends before a cross-origin Connect
// call from the web app (apps/web/src/lib/connect.ts).
func preflight(origin, requestHeaders string) *http.Request {
	req := httptest.NewRequest(http.MethodOptions, "/synthify.app.v1.TreeService/GetTree", nil)
	req.Header.Set("Origin", origin)
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", requestHeaders)
	return req
}

func failIfCalled(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next handler should not be called for a preflight")
	})
}

// headerListContains reports whether a comma-separated header list in any of
// the values contains name. Header names compare case-insensitively.
func headerListContains(values []string, name string) bool {
	for _, value := range values {
		for _, item := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(item), name) {
				return true
			}
		}
	}
	return false
}

func assertVariesOnOrigin(t *testing.T, h http.Header) {
	t.Helper()
	if !headerListContains(h.Values("Vary"), "Origin") {
		t.Fatalf("Vary = %q, want it to include Origin", h.Values("Vary"))
	}
}

func TestCORSPreflightAllowsTheWebAppRequestHeaders(t *testing.T) {
	for _, tc := range []struct {
		name      string
		requested string
	}{
		{name: "signed-in call", requested: "authorization,connect-protocol-version,content-type"},
		// features/tree/api.ts getTreeViaShareToken: the shared-link tree viewer.
		{name: "shared-link tree", requested: "connect-protocol-version,content-type,x-synthify-share-token"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			CORS(testWebOrigin, failIfCalled(t)).ServeHTTP(rr, preflight(testWebOrigin, tc.requested))

			if rr.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
			}
			if got := rr.Header().Get("Access-Control-Allow-Origin"); got != testWebOrigin {
				t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, testWebOrigin)
			}
			allowed := rr.Header().Values("Access-Control-Allow-Headers")
			for _, name := range strings.Split(tc.requested, ",") {
				if !headerListContains(allowed, name) {
					t.Errorf("Access-Control-Allow-Headers %q does not allow %q", allowed, name)
				}
			}
			assertVariesOnOrigin(t, rr.Header())
		})
	}
}

func TestCORSReflectsAnAllowedOrigin(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodPost, "/synthify.app.v1.TreeService/GetTree", nil)
	req.Header.Set("Origin", testWebOrigin)
	rr := httptest.NewRecorder()

	CORS("https://other.example.com, "+testWebOrigin, next).ServeHTTP(rr, req)

	if !called {
		t.Fatal("next handler was not called")
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != testWebOrigin {
		t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, testWebOrigin)
	}
	if got := rr.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("Access-Control-Allow-Credentials = %q, want true", got)
	}
	assertVariesOnOrigin(t, rr.Header())
}

func TestCORSDoesNotAllowOtherOrigins(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodOptions} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/synthify.app.v1.TreeService/GetTree", nil)
			req.Header.Set("Origin", "https://evil.example.com")
			rr := httptest.NewRecorder()

			CORS(testWebOrigin, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(rr, req)

			if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
				t.Fatalf("Access-Control-Allow-Origin = %q, want none", got)
			}
			assertVariesOnOrigin(t, rr.Header())
		})
	}
}

func TestCORSWildcard(t *testing.T) {
	t.Run("reflects the request origin", func(t *testing.T) {
		rr := httptest.NewRecorder()
		CORS("*", failIfCalled(t)).ServeHTTP(rr, preflight("http://localhost:5173", "content-type,x-synthify-share-token"))

		if rr.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
		}
		if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
			t.Fatalf("Access-Control-Allow-Origin = %q, want the request origin", got)
		}
		assertVariesOnOrigin(t, rr.Header())
	})
	t.Run("no origin", func(t *testing.T) {
		rr := httptest.NewRecorder()
		CORS("*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/health", nil))

		if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "*" {
			t.Fatalf("Access-Control-Allow-Origin = %q, want *", got)
		}
	})
}

func TestCORSKeepsAnEarlierVary(t *testing.T) {
	h := CORS(testWebOrigin, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	outer := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Accept-Encoding")
		h.ServeHTTP(w, r)
	})
	req := httptest.NewRequest(http.MethodPost, "/synthify.app.v1.TreeService/GetTree", nil)
	req.Header.Set("Origin", testWebOrigin)
	rr := httptest.NewRecorder()

	outer.ServeHTTP(rr, req)

	if !headerListContains(rr.Header().Values("Vary"), "Accept-Encoding") {
		t.Fatalf("Vary = %q, want it to keep Accept-Encoding", rr.Header().Values("Vary"))
	}
	assertVariesOnOrigin(t, rr.Header())
}
