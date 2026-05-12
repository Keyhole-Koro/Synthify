package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	firebase "firebase.google.com/go/v4"
	firebaseauth "firebase.google.com/go/v4/auth"
	"github.com/synthify/backend/packages/shared/applog"
)

type contextKey string

const authUserContextKey contextKey = "auth_user"
const anonymousReadAllowedContextKey contextKey = "anonymous_read_allowed"

type AuthUser struct {
	ID    string
	Email string
}

func CurrentUser(ctx context.Context) (AuthUser, bool) {
	user, ok := ctx.Value(authUserContextKey).(AuthUser)
	return user, ok
}

func AnonymousReadAllowed(ctx context.Context) bool {
	allowed, _ := ctx.Value(anonymousReadAllowedContextKey).(bool)
	return allowed
}

func WithAuth(projectID string, enableAnonymous bool, logger applog.Logger, next http.Handler) http.Handler {
	if logger == nil {
		logger = applog.NoopLogger{}
	}
	client, err := newFirebaseAuthClient(projectID)
	if err != nil {
		panic(err)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isStripeWebhookPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// allowAnonymous is primarily used by tools like log-viewer to access job/log data without per-workspace membership.
		// TODO: Re-evaluate security implications and consider a more robust service-to-service auth for these tools.
		if enableAnonymous && isAnonymousPathAllowed(r.URL.Path) {
			ctx := context.WithValue(r.Context(), anonymousReadAllowedContextKey, true)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
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
		user := AuthUser{
			ID:    idToken.UID,
			Email: email,
		}
		ctx := context.WithValue(r.Context(), authUserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func isAnonymousPathAllowed(path string) bool {
	switch path {
	case "/health",
		"/synthify.tree.v1.JobService/ListAllJobs",
		"/synthify.tree.v1.JobService/ListJobLogs",
		"/synthify.tree.v1.JobService/SearchJobLogs",
		"/synthify.tree.v1.JobService/ListRelatedJobLogs":
		return true
	default:
		return false
	}
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
