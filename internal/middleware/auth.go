package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"webapp/internal/model"
)

const authInfoKey ctxKey = "auth_info"

// jwtClaims is the minimal set of claims we expect in each token.
// PII fields (email, name, picture) were removed — they are not needed for
// authorisation and should not travel in a Base64-decodable cookie payload.
type jwtClaims struct {
	jwt.RegisteredClaims
	Roles []string `json:"roles"`
}

// SessionConfig holds the parameters required by RequireAuth for both
// token validation and transparent sliding-session renewal.
type SessionConfig struct {
	// Secret is the HMAC-SHA256 signing key (same value used at login).
	Secret []byte
	// Expiry is the total lifetime of a newly issued token (e.g. 168h).
	Expiry time.Duration
	// CookieSecure sets the Secure flag on renewed session cookies.
	// Should be true in production (HTTPS) and false for local HTTP dev.
	CookieSecure bool
}

// RequireAuth validates the access_token cookie and injects AuthInfo into the
// request context. Returns 401 with no implementation details exposed.
//
// Sliding-session renewal: if the token's remaining lifetime is less than half
// of the configured Expiry, a fresh token is issued silently as a Set-Cookie
// header on the same response. This keeps active users permanently logged in
// without requiring any client-side logic, and is especially important for
// PWA/offline scenarios where the user may be away from the network for days.
func RequireAuth(cfg SessionConfig) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("access_token")
			if err != nil {
				writeError(w, http.StatusUnauthorized, "Unauthorized")
				return
			}

			token, err := jwt.ParseWithClaims(cookie.Value, &jwtClaims{}, func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
				}
				return cfg.Secret, nil
			})
			if err != nil || !token.Valid {
				writeError(w, http.StatusUnauthorized, "Unauthorized")
				return
			}

			claims, ok := token.Claims.(*jwtClaims)
			if !ok {
				writeError(w, http.StatusUnauthorized, "Unauthorized")
				return
			}

			// Sliding renewal: transparently issue a fresh token when the
			// current one has used more than half its lifetime. The new token
			// carries identical claims so no re-login is ever required.
			if cfg.Expiry > 0 && claims.ExpiresAt != nil {
				remaining := time.Until(claims.ExpiresAt.Time)
				if remaining < cfg.Expiry/2 {
					now := time.Now()
					renewed := jwtClaims{
						RegisteredClaims: jwt.RegisteredClaims{
							Subject:   claims.Subject,
							IssuedAt:  jwt.NewNumericDate(now),
							ExpiresAt: jwt.NewNumericDate(now.Add(cfg.Expiry)),
						},
						Roles: claims.Roles,
					}
					if signed, signErr := jwt.NewWithClaims(jwt.SigningMethodHS256, renewed).SignedString(cfg.Secret); signErr == nil {
						http.SetCookie(w, &http.Cookie{
							Name:     "access_token",
							Value:    signed,
							Path:     "/",
							MaxAge:   int(cfg.Expiry.Seconds()),
							HttpOnly: true,
							Secure:   cfg.CookieSecure,
							SameSite: http.SameSiteLaxMode,
						})
					}
				}
			}

			info := model.AuthInfo{
				UserID: claims.Subject,
				Roles:  claims.Roles,
			}

			ctx := context.WithValue(r.Context(), authInfoKey, info)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole checks that the authenticated user holds at least one of the
// given roles. Must be applied after RequireAuth in the middleware chain.
// Returns 401 if no AuthInfo is present, 403 if the user lacks all listed roles.
func RequireRole(allowed ...string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			info, ok := GetAuthInfo(r.Context())
			if !ok {
				writeError(w, http.StatusUnauthorized, "Unauthorized")
				return
			}
			if !info.HasAnyRole(allowed...) {
				writeError(w, http.StatusForbidden, "Forbidden")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// GetAuthInfo extracts the authenticated user from the request context.
func GetAuthInfo(ctx context.Context) (model.AuthInfo, bool) {
	info, ok := ctx.Value(authInfoKey).(model.AuthInfo)
	return info, ok
}

// WithAuthInfo injects AuthInfo into the context (useful for testing).
func WithAuthInfo(ctx context.Context, info model.AuthInfo) context.Context {
	return context.WithValue(ctx, authInfoKey, info)
}

func writeError(w http.ResponseWriter, status int, title string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"type":"about:blank","title":%q,"status":%d}`, title, status)
}
