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

		// 公開リンク経由の閲覧: share token を持ち、対象が share でアクセス可能な
		// read RPC なら、bearer 無しでも通す。token は context に載せ、各 service が
		// token -> workspace の認可を行う (default-deny は service 側の責務)。
		if shareToken := strings.TrimSpace(r.Header.Get("X-Synthify-Share-Token")); shareToken != "" && isShareReadable(r) {
			ctx := apiauth.ContextWithShareToken(r.Context(), shareToken)
			next.ServeHTTP(w, r.WithContext(ctx))
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
	switch r.URL.Path {
	case "/health", "/stripe/webhook":
		return true
	// 公開リンクの解決は無認証経路 (token のみで workspace を解決する)。
	case "/synthify.app.v1.WorkspaceService/ResolveShareLink":
		return true
	}
	return false
}

// isShareReadable は share token で読める read 専用 RPC かを返す。
// 書き込み RPC は含めない (公開リンクは閲覧専用)。
func isShareReadable(r *http.Request) bool {
	switch r.URL.Path {
	case "/synthify.app.v1.TreeService/GetTree",
		"/synthify.app.v1.TreeService/GetSubtree",
		"/synthify.app.v1.TreeService/FindPaths":
		return true
	}
	return false
}

func bearerToken(header string) string {
	const prefix = "bearer "
	if !strings.HasPrefix(strings.ToLower(header), prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}
