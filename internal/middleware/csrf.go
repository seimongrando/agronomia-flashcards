package middleware

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// CSRF validates the Origin or Referer header on state-mutating requests
// (POST, PUT, PATCH, DELETE) to defend against cross-site request forgery.
//
// The check is skipped for:
//   - Non-mutating methods (GET, HEAD, OPTIONS).
//   - The OAuth callback path (/auth/google/callback) which arrives via
//     browser redirect, not an XHR.
//
// In development (isDev=true) a missing or empty origin is allowed so that
// curl / Postman testing does not break. In production an absent origin is
// rejected.
//
// allowedOrigins is the explicit whitelist. If it is empty the middleware
// accepts any request whose Origin matches the Host of the incoming request
// (same-site). The frontend already adds X-Requested-With on every API call;
// this is a defence-in-depth check at the HTTP layer.
func CSRF(allowedOrigins []string, isDev bool) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isMutatingMethod(r.Method) || isCSRFExemptPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			origin := extractOrigin(r)

			if origin == "" {
				if isDev {
					next.ServeHTTP(w, r)
					return
				}
				csrfError(w)
				return
			}

			if originAllowed(origin, allowedOrigins, r) {
				next.ServeHTTP(w, r)
				return
			}

			csrfError(w)
		})
	}
}

func isMutatingMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

// isCSRFExemptPath lists paths that arrive as plain browser redirects, not XHR.
func isCSRFExemptPath(path string) bool {
	return strings.HasPrefix(path, "/auth/google")
}

// extractOrigin returns the scheme+host from the Origin header; if absent it
// falls back to parsing the Referer header.
func extractOrigin(r *http.Request) string {
	if o := r.Header.Get("Origin"); o != "" {
		return strings.TrimRight(o, "/")
	}
	if ref := r.Header.Get("Referer"); ref != "" {
		if u, err := url.Parse(ref); err == nil && u.Host != "" {
			return fmt.Sprintf("%s://%s", u.Scheme, u.Host)
		}
	}
	return ""
}

// originAllowed returns true when origin is in the explicit whitelist, or —
// when the whitelist is empty — when origin matches the server's own host.
func originAllowed(origin string, allowed []string, r *http.Request) bool {
	if len(allowed) > 0 {
		for _, a := range allowed {
			if strings.EqualFold(strings.TrimRight(a, "/"), origin) {
				return true
			}
		}
		return false
	}

	// No explicit list: accept same-site (origin == scheme+host of this server).
	host := r.Host
	if host == "" {
		return false
	}
	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	return strings.EqualFold(origin, fmt.Sprintf("%s://%s", scheme, host))
}

func csrfError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`{"type":"about:blank","title":"Forbidden","status":403,"detail":"invalid or missing origin"}`))
}
