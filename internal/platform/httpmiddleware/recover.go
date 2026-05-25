package httpmiddleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recover catches panics, logs the stack trace, and returns 500.
func Recover(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				logger.Error("panic.recovered",
					"error", fmt.Sprintf("panic: %v", rec),
					"stack", string(debug.Stack()),
					"path", r.URL.Path,
					"method", r.Method,
				)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
