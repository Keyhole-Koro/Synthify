package middleware

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"os"
	"strings"

	firebase "firebase.google.com/go/v4"
	firebaseauth "firebase.google.com/go/v4/auth"
	"github.com/synthify/backend/packages/shared/applog"
)

type contextKey string

const authUserContextKey contextKey = "auth_user"
const serviceCallContextKey contextKey = "service_call"
const adminUserContextKey contextKey = "admin_user"

type AuthUser struct {
	ID    string
	Email string
}

func CurrentUser(ctx context.Context) (AuthUser, bool) {
	user, ok := ctx.Value(authUserContextKey).(AuthUser)
	return user, ok
}

// IsServiceCall は worker -> API のような内部サービス間呼び出しを示す。
// service token (環境変数 SYNTHIFY_INTERNAL_SERVICE_TOKEN) が一致した場合のみ true。
func IsServiceCall(ctx context.Context) bool {
	allowed, _ := ctx.Value(serviceCallContextKey).(bool)
	return allowed
}

// IsAdmin は管理者権限を持つユーザーのリクエストか。
// 現状は ADMIN_USER_EMAILS の allowlist で判定する暫定実装。
func IsAdmin(ctx context.Context) bool {
	admin, _ := ctx.Value(adminUserContextKey).(bool)
	return admin
}

// ContextWithServiceCall はテストおよび手動構築用に service call フラグを立てる。
func ContextWithServiceCall(ctx context.Context) context.Context {
	return context.WithValue(ctx, serviceCallContextKey, true)
}

// ContextWithAdmin はテストおよび手動構築用に admin フラグを立てる。
func ContextWithAdmin(ctx context.Context, user AuthUser) context.Context {
	ctx = context.WithValue(ctx, authUserContextKey, user)
	return context.WithValue(ctx, adminUserContextKey, true)
}

func WithAuth(projectID string, logger applog.Logger, next http.Handler) http.Handler {
	if logger == nil {
		logger = applog.NoopLogger{}
	}
	client, err := newFirebaseAuthClient(projectID)
	if err != nil {
		panic(err)
	}

	expectedServiceToken := strings.TrimSpace(os.Getenv("SYNTHIFY_INTERNAL_SERVICE_TOKEN"))
	adminEmails := parseEmailSet(os.Getenv("SYNTHIFY_ADMIN_USER_EMAILS"))
	// Access allowlist. When non-empty, only these emails may reach the API
	// (used to lock stage to a single account). Empty => no restriction, so
	// prod behaviour is unchanged. Service calls and the Stripe webhook are
	// exempt because they return earlier, before this check.
	allowedEmails := parseEmailSet(os.Getenv("SYNTHIFY_ALLOWED_USER_EMAILS"))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// OPTIONS requests (CORS preflight) should never require authentication.
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		if isStripeWebhookPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// 内部サービス間呼び出し (worker -> API 等)。SYNTHIFY_INTERNAL_SERVICE_TOKEN が
		// 設定されていて、リクエストの X-Synthify-Service-Token がそれと一致する場合のみ通す。
		if expectedServiceToken != "" {
			if got := r.Header.Get("X-Synthify-Service-Token"); got != "" {
				if subtle.ConstantTimeCompare([]byte(got), []byte(expectedServiceToken)) == 1 {
					ctx := context.WithValue(r.Context(), serviceCallContextKey, true)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				logger.Warn(r.Context(), "auth.service_token_mismatch", nil, map[string]any{"path": r.URL.Path})
				http.Error(w, "invalid service token", http.StatusUnauthorized)
				return
			}
		}

		authHeader := r.Header.Get("Authorization")
		token := bearerToken(authHeader)
		if token == "" {
			logger.Warn(r.Context(), "auth.missing_token", nil, map[string]any{"path": r.URL.Path})
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}

		idToken, err := client.VerifyIDToken(r.Context(), token)
		if err != nil {
			logger.Warn(r.Context(), "auth.verify_failed", err, map[string]any{"path": r.URL.Path})
			http.Error(w, "invalid bearer token", http.StatusUnauthorized)
			return
		}

		email, _ := idToken.Claims["email"].(string)

		// Enforce the access allowlist when configured.
		if len(allowedEmails) > 0 && !allowedEmails[lowerTrim(email)] {
			logger.Warn(r.Context(), "auth.email_not_allowed", nil, map[string]any{
				"path":  r.URL.Path,
				"email": email,
			})
			http.Error(w, "access restricted to allowed users", http.StatusForbidden)
			return
		}

		user := AuthUser{
			ID:    idToken.UID,
			Email: email,
		}
		ctx := context.WithValue(r.Context(), authUserContextKey, user)
		if email != "" && adminEmails[strings.ToLower(email)] {
			ctx = context.WithValue(ctx, adminUserContextKey, true)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func lowerTrim(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func parseEmailSet(csv string) map[string]bool {
	result := map[string]bool{}
	for _, raw := range strings.Split(csv, ",") {
		if e := lowerTrim(raw); e != "" {
			result[e] = true
		}
	}
	return result
}

func isStripeWebhookPath(path string) bool {
	return path == "/stripe/webhook"
}

func newFirebaseAuthClient(projectID string) (*firebaseauth.Client, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, errors.New("FIREBASE_PROJECT_ID is required")
	}
	app, err := firebase.NewApp(context.Background(), &firebase.Config{ProjectID: projectID})
	if err != nil {
		return nil, err
	}
	return app.Auth(context.Background())
}

// ContextWithUser returns a copy of ctx carrying user.
// Intended for use in tests and integration utilities.
func ContextWithUser(ctx context.Context, user AuthUser) context.Context {
	return context.WithValue(ctx, authUserContextKey, user)
}

func bearerToken(header string) string {
	const prefix = "bearer "
	if !strings.HasPrefix(strings.ToLower(header), prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}
