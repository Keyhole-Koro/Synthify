package auth

import (
	"context"
	"errors"

	connect "connectrpc.com/connect"
)

// PrincipalKind identifies the classification of an authenticated caller.
type PrincipalKind string

const (
	// PrincipalKindUser identifies an interactive end user authenticated via Firebase.
	PrincipalKindUser PrincipalKind = "user"
	// PrincipalKindService identifies an automated internal service caller authenticated via shared token.
	PrincipalKindService PrincipalKind = "service"
)

// Principal represents the authenticated security context of an API request.
type Principal struct {
	Kind        PrincipalKind
	SubjectID   string
	Email       string
	DisplayName string
	Admin       bool
}

// Authenticator defines the contract for authenticating bearer tokens and service secrets.
type Authenticator interface {
	AuthenticateBearer(ctx context.Context, token string) (Principal, error)
	AuthenticateServiceToken(ctx context.Context, token string) (Principal, error)
}

type contextKey string

const principalContextKey contextKey = "principal"
const shareTokenContextKey contextKey = "share_token"

// ContextWithPrincipal attaches an authenticated Principal to the context.
func ContextWithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalContextKey, principal)
}

// PrincipalFromContext retrieves the authenticated Principal from the context, if present.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey).(Principal)
	return principal, ok
}

// ContextWithShareToken attaches a public workspace share token to the request context.
// Used by unauthenticated guest viewers (e.g. read-only tree item viewing).
func ContextWithShareToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, shareTokenContextKey, token)
}

// ShareTokenFromContext extracts any public workspace share token present in the request context.
func ShareTokenFromContext(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(shareTokenContextKey).(string)
	return token, ok && token != ""
}

// RequirePrincipal asserts that a valid Principal exists on the context, returning an unauthenticated error if absent.
func RequirePrincipal(ctx context.Context) (Principal, error) {
	principal, ok := PrincipalFromContext(ctx)
	if !ok {
		return Principal{}, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}
	return principal, nil
}

// RequireUserPrincipal asserts that the request principal is an authenticated interactive user.
func RequireUserPrincipal(ctx context.Context) (Principal, error) {
	principal, err := RequirePrincipal(ctx)
	if err != nil {
		return Principal{}, err
	}
	if principal.Kind != PrincipalKindUser || principal.SubjectID == "" {
		return Principal{}, connect.NewError(connect.CodeUnauthenticated, errors.New("user authentication required"))
	}
	return principal, nil
}

// RequireServicePrincipal asserts that the request principal is an authenticated internal service.
func RequireServicePrincipal(ctx context.Context) (Principal, error) {
	principal, err := RequirePrincipal(ctx)
	if err != nil {
		return Principal{}, err
	}
	if principal.Kind != PrincipalKindService || principal.SubjectID == "" {
		return Principal{}, connect.NewError(connect.CodePermissionDenied, errors.New("service token required"))
	}
	return principal, nil
}

// RequireAdminPrincipal asserts that the request principal has administrator rights.
func RequireAdminPrincipal(ctx context.Context) (Principal, error) {
	principal, err := RequireUserPrincipal(ctx)
	if err != nil {
		return Principal{}, err
	}
	if !principal.Admin {
		return Principal{}, connect.NewError(connect.CodePermissionDenied, errors.New("admin role required"))
	}
	return principal, nil
}

// RequireUserID extracts the authenticated user's SubjectID, returning an error if unauthenticated or not a user.
func RequireUserID(ctx context.Context) (string, error) {
	principal, err := RequireUserPrincipal(ctx)
	if err != nil {
		return "", err
	}
	return principal.SubjectID, nil
}
