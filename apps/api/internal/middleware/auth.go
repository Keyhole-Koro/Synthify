package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	apiauth "github.com/synthify/backend/apps/api/internal/auth"
)

func WithAuth(authenticator apiauth.Authenticator, logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isAuthExempt(r) {
			next.ServeHTTP(w, r)
			return
		}

		if serviceToken := strings.TrimSpace(r.Header.Get("X-Synthify-Service-Token")); serviceToken != "" {
			principal, err := authenticator.AuthenticateServiceToken(r.Context(), serviceToken)
			if err != nil {
				logger.Warn("auth.service_token_failed", "error", err.Error(), "path", r.URL.Path)
				writeAuthError(w, err, "invalid service token")
				return
			}
			next.ServeHTTP(w, r.WithContext(apiauth.ContextWithPrincipal(r.Context(), principal)))
			return
		}

		token := bearerToken(r.Header.Get("Authorization"))
		if token == "" {
			logger.Warn("auth.missing_token", "path", r.URL.Path)
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}
		principal, err := authenticator.AuthenticateBearer(r.Context(), token)
		if err != nil {
			logger.Warn("auth.bearer_failed", "error", err.Error(), "path", r.URL.Path)
			writeAuthError(w, err, "invalid bearer token")
			return
		}
		next.ServeHTTP(w, r.WithContext(apiauth.ContextWithPrincipal(r.Context(), principal)))
	})
}

func writeAuthError(w http.ResponseWriter, err error, unauthorizedMessage string) {
	if errors.Is(err, apiauth.ErrPermissionDenied) {
		http.Error(w, "access restricted to allowed users", http.StatusForbidden)
		return
	}
	http.Error(w, unauthorizedMessage, http.StatusUnauthorized)
}

func isAuthExempt(r *http.Request) bool {
	if r.Method == http.MethodOptions {
		return true
	}
	return r.URL.Path == "/health" || r.URL.Path == "/stripe/webhook"
}

func bearerToken(header string) string {
	const prefix = "bearer "
	if !strings.HasPrefix(strings.ToLower(header), prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}
