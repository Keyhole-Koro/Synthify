package auth

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/api/idtoken"
)

func serviceAccountPayload(email string, verified bool) *idtoken.Payload {
	return &idtoken.Payload{
		Claims: map[string]interface{}{
			"email":          email,
			"email_verified": verified,
		},
	}
}

func stubValidator(payload *idtoken.Payload, err error) IDTokenValidator {
	return func(context.Context, string, string) (*idtoken.Payload, error) {
		return payload, err
	}
}

func newTestAuthenticator(t *testing.T, allowed string, validator IDTokenValidator) *SchedulerAuthenticator {
	t.Helper()
	a, err := NewSchedulerAuthenticator(SchedulerAuthenticatorConfig{
		Audience:                  "https://api.example.com",
		AllowedServiceAccountsCSV: allowed,
		Validator:                 validator,
	})
	if err != nil {
		t.Fatalf("NewSchedulerAuthenticator: %v", err)
	}
	return a
}

func TestNewSchedulerAuthenticator_FailsClosedWithoutConfig(t *testing.T) {
	for _, tc := range []struct {
		name     string
		audience string
		allowed  string
	}{
		{name: "no audience", allowed: "sched@example.iam.gserviceaccount.com"},
		{name: "no allowlist", audience: "https://api.example.com"},
		{name: "allowlist of blanks", audience: "https://api.example.com", allowed: " , "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewSchedulerAuthenticator(SchedulerAuthenticatorConfig{
				Audience:                  tc.audience,
				AllowedServiceAccountsCSV: tc.allowed,
			}); err == nil {
				t.Fatal("expected an error so the endpoint is never mounted half-configured")
			}
		})
	}
}

func TestSchedulerAuthenticator_AcceptsAllowlistedServiceAccount(t *testing.T) {
	const email = "Sched@example.iam.gserviceaccount.com"
	a := newTestAuthenticator(t, "sched@example.iam.gserviceaccount.com",
		stubValidator(serviceAccountPayload(email, true), nil))

	caller, err := a.Authenticate(context.Background(), "Bearer token")
	if err != nil {
		t.Fatalf("expected the allowlisted caller to authenticate, got %v", err)
	}
	if caller != "sched@example.iam.gserviceaccount.com" {
		t.Fatalf("expected the normalized email, got %q", caller)
	}
}

func TestSchedulerAuthenticator_PassesConfiguredAudienceToValidator(t *testing.T) {
	var gotAudience, gotToken string
	a := newTestAuthenticator(t, "sched@example.iam.gserviceaccount.com",
		func(_ context.Context, token, audience string) (*idtoken.Payload, error) {
			gotToken, gotAudience = token, audience
			return serviceAccountPayload("sched@example.iam.gserviceaccount.com", true), nil
		})

	if _, err := a.Authenticate(context.Background(), "Bearer abc123"); err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if gotToken != "abc123" {
		t.Fatalf("expected the bearer value without the prefix, got %q", gotToken)
	}
	if gotAudience != "https://api.example.com" {
		t.Fatalf("expected the configured audience, got %q", gotAudience)
	}
}

func TestSchedulerAuthenticator_RejectsUnauthenticated(t *testing.T) {
	for _, tc := range []struct {
		name    string
		header  string
		payload *idtoken.Payload
		err     error
	}{
		{name: "missing header", payload: serviceAccountPayload("sched@example.iam.gserviceaccount.com", true)},
		{name: "not a bearer header", header: "Basic abc", payload: serviceAccountPayload("sched@example.iam.gserviceaccount.com", true)},
		{name: "invalid token", header: "Bearer abc", err: errors.New("audience mismatch")},
		{name: "no email claim", header: "Bearer abc", payload: serviceAccountPayload("", true)},
		{name: "unverified email", header: "Bearer abc", payload: serviceAccountPayload("sched@example.iam.gserviceaccount.com", false)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newTestAuthenticator(t, "sched@example.iam.gserviceaccount.com", stubValidator(tc.payload, tc.err))

			_, err := a.Authenticate(context.Background(), tc.header)
			if !errors.Is(err, ErrUnauthenticated) {
				t.Fatalf("expected ErrUnauthenticated, got %v", err)
			}
		})
	}
}

func TestSchedulerAuthenticator_RejectsValidTokenFromOtherIdentity(t *testing.T) {
	a := newTestAuthenticator(t, "sched@example.iam.gserviceaccount.com",
		stubValidator(serviceAccountPayload("someone-else@example.iam.gserviceaccount.com", true), nil))

	_, err := a.Authenticate(context.Background(), "Bearer abc")
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied so the handler answers 403, got %v", err)
	}
}
