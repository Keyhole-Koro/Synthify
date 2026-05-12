package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/synthify/backend/packages/shared/applog"
)

// CORS adds CORS headers for allowed origins.
// Preflight (OPTIONS) requests are terminated here.
func CORS(allowedOrigins string, next http.Handler) http.Handler {
	allowed := make(map[string]bool)
	for _, o := range strings.Split(allowedOrigins, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			allowed[o] = true
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowed[origin] || allowedOrigins == "*" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		} else if len(allowed) == 0 {
			// Development fallback.
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Logger logs the request method, path, status, and response time.
func Logger(logger applog.Logger, next http.Handler) http.Handler {
	if logger == nil {
		logger = applog.NoopLogger{}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		elapsed := time.Since(start)

		fields := map[string]any{
			"method":  r.Method,
			"path":    r.URL.Path,
			"status":  rw.status,
			"elapsed": elapsed.String(),
		}

		if rw.status >= 500 {
			logger.Error(r.Context(), "http.request", nil, fields)
		} else {
			logger.Info(r.Context(), "http.request", fields)
		}

		if elapsed > 5*time.Second {
			logger.Warn(r.Context(), "http.request.slow", nil, fields)
		}
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}
