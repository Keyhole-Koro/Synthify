package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	apiauth "github.com/synthify/backend/apps/api/internal/auth"
)

func TestBearerToken(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"valid bearer token", "Bearer abc123", "abc123"},
		{"lowercase bearer also valid", "bearer abc", "abc"},
		{"missing prefix", "abc123", ""},
		{"empty header", "", ""},
		{"value trimmed of spaces", "Bearer  spaced ", "spaced"},
		{"bearer with no value", "Bearer ", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := bearerToken(tc.header)
			if got != tc.want {
				t.Errorf("bearerToken(%q) = %q, want %q", tc.header, got, tc.want)
			}
		})
	}
}

func TestWithAuth_ExemptPathSkipsAuthenticator(t *testing.T) {
	authenticator := &fakeAuthenticator{}
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	WithAuth(authenticator, slog.Default(), next).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if !called {
		t.Fatal("next handler was not called")
	}
	if authenticator.bearerCalls != 0 || authenticator.serviceCalls != 0 {
		t.Fatalf("authenticator called: bearer=%d service=%d", authenticator.bearerCalls, authenticator.serviceCalls)
	}
}

func TestWithAuth_BearerTokenUsesBearerAuthenticator(t *testing.T) {
	want := apiauth.Principal{Kind: apiauth.PrincipalKindUser, SubjectID: "u1", Email: "u@example.com"}
	authenticator := &fakeAuthenticator{bearerPrincipal: want}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok := apiauth.PrincipalFromContext(r.Context())
		if !ok || got != want {
			t.Fatalf("principal = %+v ok=%v, want %+v", got, ok, want)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/rpc", nil)
	req.Header.Set("Authorization", "Bearer token-1")
	rec := httptest.NewRecorder()
	WithAuth(authenticator, slog.Default(), next).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if authenticator.gotBearer != "token-1" || authenticator.serviceCalls != 0 {
		t.Fatalf("got bearer=%q serviceCalls=%d", authenticator.gotBearer, authenticator.serviceCalls)
	}
}

func TestWithAuth_ServiceTokenUsesServiceAuthenticator(t *testing.T) {
	want := apiauth.Principal{Kind: apiauth.PrincipalKindService, SubjectID: apiauth.InternalServiceSubjectID}
	authenticator := &fakeAuthenticator{servicePrincipal: want}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok := apiauth.PrincipalFromContext(r.Context())
		if !ok || got != want {
			t.Fatalf("principal = %+v ok=%v, want %+v", got, ok, want)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/rpc", nil)
	req.Header.Set("X-Synthify-Service-Token", "svc-token")
	rec := httptest.NewRecorder()
	WithAuth(authenticator, slog.Default(), next).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if authenticator.gotService != "svc-token" || authenticator.bearerCalls != 0 {
		t.Fatalf("got service=%q bearerCalls=%d", authenticator.gotService, authenticator.bearerCalls)
	}
}

func TestWithAuth_MissingTokenReturnsUnauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/rpc", nil)
	rec := httptest.NewRecorder()
	WithAuth(&fakeAuthenticator{}, slog.Default(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestWithAuth_PermissionDeniedReturnsForbidden(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/rpc", nil)
	req.Header.Set("Authorization", "Bearer token-1")
	rec := httptest.NewRecorder()
	WithAuth(&fakeAuthenticator{bearerErr: apiauth.ErrPermissionDenied}, slog.Default(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

type fakeAuthenticator struct {
	bearerPrincipal  apiauth.Principal
	servicePrincipal apiauth.Principal
	bearerErr        error
	serviceErr       error
	bearerCalls      int
	serviceCalls     int
	gotBearer        string
	gotService       string
}

func (f *fakeAuthenticator) AuthenticateBearer(ctx context.Context, token string) (apiauth.Principal, error) {
	_ = ctx
	f.bearerCalls++
	f.gotBearer = token
	if f.bearerErr != nil {
		return apiauth.Principal{}, f.bearerErr
	}
	if f.bearerPrincipal == (apiauth.Principal{}) {
		return apiauth.Principal{}, errors.New("missing bearer principal")
	}
	return f.bearerPrincipal, nil
}

func (f *fakeAuthenticator) AuthenticateServiceToken(ctx context.Context, token string) (apiauth.Principal, error) {
	_ = ctx
	f.serviceCalls++
	f.gotService = token
	if f.serviceErr != nil {
		return apiauth.Principal{}, f.serviceErr
	}
	if f.servicePrincipal == (apiauth.Principal{}) {
		return apiauth.Principal{}, errors.New("missing service principal")
	}
	return f.servicePrincipal, nil
}
