package middleware

import "net/http"

const (
	headerContentTypeOptions   = "X-Content-Type-Options"
	headerReferrerPolicy       = "Referrer-Policy"
	headerStrictTransport      = "Strict-Transport-Security"
	contentTypeOptionsNoSniff  = "nosniff"
	referrerPolicyStrictOrigin = "strict-origin-when-cross-origin"
	strictTransportOneYear     = "max-age=31536000; includeSubDomains"
)

// SecurityHeaders adds baseline security response headers for API responses.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set(headerContentTypeOptions, contentTypeOptionsNoSniff)
		h.Set(headerReferrerPolicy, referrerPolicyStrictOrigin)
		h.Set(headerStrictTransport, strictTransportOneYear)

		next.ServeHTTP(w, r)
	})
}
