package middleware

import "net/http"

const (
	// headerContentTypeOptions is the header key for MIME-type sniffing mitigation.
	headerContentTypeOptions = "X-Content-Type-Options"
	// headerReferrerPolicy is the header key controlling the Referer header in outbound requests.
	headerReferrerPolicy = "Referrer-Policy"
	// headerStrictTransport is the header key for HTTP Strict Transport Security.
	headerStrictTransport = "Strict-Transport-Security"
	// contentTypeOptionsNoSniff prevents the browser from MIME-sniffing away from declared content-type.
	contentTypeOptionsNoSniff = "nosniff"
	// referrerPolicyStrictOrigin sends full referrer only for same-origin, origin only for cross-origin HTTPS.
	referrerPolicyStrictOrigin = "strict-origin-when-cross-origin"
	// strictTransportOneYear enforces HTTPS for one year including subdomains.
	strictTransportOneYear = "max-age=31536000; includeSubDomains"
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
