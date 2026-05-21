package httpmiddleware

import (
	"net/http"
	"time"

	"github.com/synthify/backend/internal/platform/applog"
)

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
