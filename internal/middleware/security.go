package middleware

import (
	"net/http"
	"strings"
)

// SecurityHeaders returns a middleware that sets hardened HTTP response headers
// on every response. Pass prod=true in production to also emit HSTS.
//
// Content-Security-Policy notes:
//   - script-src 'self' only — no inline scripts.
//   - style-src 'self' 'unsafe-inline' — kept because the frontend uses
//     inline style attributes (e.g. style="display:none"). The risk is low
//     compared to allowing unsafe-inline scripts.
//   - frame-ancestors 'none' replaces the legacy X-Frame-Options header and
//     is understood by modern browsers; X-Frame-Options: DENY is kept for
//     older browser compat.
func SecurityHeaders(prod bool) Middleware {
	const csp = "default-src 'self';" +
		" script-src 'self';" +
		" style-src 'self' 'unsafe-inline' https://fonts.googleapis.com;" +
		" img-src 'self' https://lh3.googleusercontent.com data:;" +
		" connect-src 'self';" +
		" font-src 'self' https://fonts.gstatic.com;" +
		" object-src 'none';" +
		" frame-ancestors 'none';" +
		" base-uri 'self';" +
		" form-action 'self' https://accounts.google.com"

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("Content-Security-Policy", csp)
			h.Set("X-Frame-Options", "DENY")
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
			h.Set("X-XSS-Protection", "0") // CSP supersedes this; "0" disables buggy browser behaviour
			if prod {
				// HSTS: tells browsers to always use HTTPS for the next year.
				// Only set in production where TLS is guaranteed (OWASP A02).
				h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// MaxBody limits JSON/text request bodies. Multipart requests (file uploads)
// are exempt so handlers can enforce their own per-endpoint size limits via
// http.MaxBytesReader + ParseMultipartForm.
func MaxBody(maxBytes int64) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				ct := r.Header.Get("Content-Type")
				if !strings.HasPrefix(ct, "multipart/") {
					r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
