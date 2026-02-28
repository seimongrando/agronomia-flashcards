package middleware

import (
	"context"
	"fmt"
	"net/http"

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

// RequireAuth validates the access_token cookie and injects AuthInfo into the
// request context. Returns 401 with no implementation details exposed.
func RequireAuth(jwtSecret []byte) Middleware {
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
				return jwtSecret, nil
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
