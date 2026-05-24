package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeadersAddsBaselineHeaders(t *testing.T) {
	h := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/test", nil))

	assertSecurityHeaders(t, rr.Result().Header)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
	}
}

func TestSecurityHeadersWithCORSPreflight(t *testing.T) {
	h := SecurityHeaders(CORS("https://app.example.com", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called for preflight")
	})))

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "https://app.example.com")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assertSecurityHeaders(t, rr.Result().Header)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
}

func assertSecurityHeaders(t *testing.T, h http.Header) {
	t.Helper()
	tests := map[string]string{
		headerContentTypeOptions: contentTypeOptionsNoSniff,
		headerReferrerPolicy:     referrerPolicyStrictOrigin,
		headerStrictTransport:    strictTransportOneYear,
	}
	for key, want := range tests {
		if got := h.Get(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}
