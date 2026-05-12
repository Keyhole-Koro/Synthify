package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/synthify/backend/packages/shared/applog"
)

// Recover catches panics, logs the stack trace, and returns 500.
func Recover(logger applog.Logger, next http.Handler) http.Handler {
	if logger == nil {
		logger = applog.NoopLogger{}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				logger.Error(r.Context(), "panic.recovered", fmt.Errorf("panic: %v", rec), map[string]any{
					"stack":  string(debug.Stack()),
					"path":   r.URL.Path,
					"method": r.Method,
				})
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
