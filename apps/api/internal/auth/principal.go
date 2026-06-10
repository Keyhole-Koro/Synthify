package auth

import (
	"context"
	"errors"

	connect "connectrpc.com/connect"
)

type PrincipalKind string

const (
	PrincipalKindUser    PrincipalKind = "user"
	PrincipalKindService PrincipalKind = "service"
)

type Principal struct {
	Kind        PrincipalKind
	SubjectID   string
	Email       string
	DisplayName string
	Admin       bool
}

type Authenticator interface {
	AuthenticateBearer(ctx context.Context, token string) (Principal, error)
	AuthenticateServiceToken(ctx context.Context, token string) (Principal, error)
}

type contextKey string

const principalContextKey contextKey = "principal"
const shareTokenContextKey contextKey = "share_token"

func ContextWithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalContextKey, principal)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey).(Principal)
	return principal, ok
}

// ContextWithShareToken は公開リンク token を context に載せる。
// 無認証の閲覧経路 (tree の read 等) が token 認可に使う。
func ContextWithShareToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, shareTokenContextKey, token)
}

// ShareTokenFromContext は載っている公開リンク token を返す。無ければ ("", false)。
func ShareTokenFromContext(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(shareTokenContextKey).(string)
	return token, ok && token != ""
}

func RequirePrincipal(ctx context.Context) (Principal, error) {
	principal, ok := PrincipalFromContext(ctx)
	if !ok {
		return Principal{}, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}
	return principal, nil
}

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

func RequireUserID(ctx context.Context) (string, error) {
	principal, err := RequireUserPrincipal(ctx)
	if err != nil {
		return "", err
	}
	return principal.SubjectID, nil
}
