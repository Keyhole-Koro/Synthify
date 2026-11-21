package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/api/idtoken"
)

// IDTokenValidator validates a Google-issued OIDC ID token against an expected
// audience. Injected so tests do not have to reach Google's cert endpoint.
type IDTokenValidator func(ctx context.Context, token, audience string) (*idtoken.Payload, error)

type SchedulerAuthenticatorConfig struct {
	// Audience the OIDC token must carry. Cloud Scheduler mints the token with
	// the audience configured on the job; terraform sets it to the API's Cloud
	// Run URL. Empty => the authenticator fails closed.
	Audience string
	// AllowedServiceAccountsCSV lists the service account emails permitted to
	// call. Empty => fails closed, same posture as the admin allowlist.
	AllowedServiceAccountsCSV string
	// Validator defaults to idtoken.Validate.
	Validator IDTokenValidator
}

// SchedulerAuthenticator authenticates Cloud Scheduler (or any Google OIDC)
// callers on endpoints the Firebase middleware exempts. The API service is
// deployed with allow_unauthenticated=true so Cloud Run IAM cannot gate these
// routes; the check has to happen here.
type SchedulerAuthenticator struct {
	validate      IDTokenValidator
	audience      string
	allowedEmails map[string]bool
}

func NewSchedulerAuthenticator(cfg SchedulerAuthenticatorConfig) (*SchedulerAuthenticator, error) {
	audience := strings.TrimSpace(cfg.Audience)
	if audience == "" {
		return nil, errors.New("scheduler authenticator: audience is required")
	}
	allowed := parseEmailSet(cfg.AllowedServiceAccountsCSV)
	if len(allowed) == 0 {
		return nil, errors.New("scheduler authenticator: at least one allowed service account is required")
	}
	validate := cfg.Validator
	if validate == nil {
		validate = idtoken.Validate
	}
	return &SchedulerAuthenticator{validate: validate, audience: audience, allowedEmails: allowed}, nil
}

// Authenticate verifies an Authorization header carrying a Google OIDC bearer
// token. It returns ErrUnauthenticated when the token is absent or invalid and
// ErrPermissionDenied when the token is valid but the caller is not allowlisted,
// so the handler can map them to 401 vs 403.
func (a *SchedulerAuthenticator) Authenticate(ctx context.Context, authorizationHeader string) (string, error) {
	token := bearerToken(authorizationHeader)
	if token == "" {
		return "", fmt.Errorf("%w: missing bearer token", ErrUnauthenticated)
	}
	payload, err := a.validate(ctx, token, a.audience)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrUnauthenticated, err)
	}
	rawEmail, _ := payload.Claims["email"].(string)
	email := lowerTrim(rawEmail)
	if email == "" {
		return "", fmt.Errorf("%w: token carries no email claim", ErrUnauthenticated)
	}
	// Google mints email_verified=true for service-account identities; a token
	// without it is not one, so refuse rather than trust the email claim.
	if verified, ok := payload.Claims["email_verified"].(bool); !ok || !verified {
		return "", fmt.Errorf("%w: token email is not verified", ErrUnauthenticated)
	}
	if !a.allowedEmails[email] {
		return "", fmt.Errorf("%w: %s is not an allowed caller", ErrPermissionDenied, email)
	}
	return email, nil
}

func bearerToken(header string) string {
	const prefix = "bearer "
	if !strings.HasPrefix(strings.ToLower(header), prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}
