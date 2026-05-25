package httpmiddleware

import (
	"log/slog"
	"net/http"
	"time"
)

// Logger logs the request method, path, status, and response time.
func Logger(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		elapsed := time.Since(start)

		attrs := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"elapsed", elapsed.String(),
		}

		if rw.status >= 500 {
			logger.Error("http.request", attrs...)
		} else {
			logger.Info("http.request", attrs...)
		}

		if elapsed > 5*time.Second {
			logger.Warn("http.request.slow", attrs...)
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
