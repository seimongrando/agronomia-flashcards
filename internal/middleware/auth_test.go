package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"webapp/internal/model"
)

var testSecret = []byte("test-secret-key-at-least-32-bytes!")

func makeToken(t *testing.T, secret []byte, userID string, roles []string, ttl time.Duration) string {
	t.Helper()
	claims := jwtClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		},
		Email: "test@example.com",
		Name:  "Test User",
		Roles: roles,
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
	if err != nil {
		t.Fatalf("makeToken: %v", err)
	}
	return signed
}

var okHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
})

// ---------------------------------------------------------------------------
// RequireAuth
// ---------------------------------------------------------------------------

func TestRequireAuth(t *testing.T) {
	wrongSecret := []byte("wrong-secret-key-also-32-bytes!!")

	tests := []struct {
		name   string
		cookie *http.Cookie
		want   int
	}{
		{
			name:   "no cookie → 401",
			cookie: nil,
			want:   http.StatusUnauthorized,
		},
		{
			name:   "garbage token → 401",
			cookie: &http.Cookie{Name: "access_token", Value: "not.a.jwt"},
			want:   http.StatusUnauthorized,
		},
		{
			name:   "expired token → 401",
			cookie: &http.Cookie{Name: "access_token", Value: makeToken(t, testSecret, "u1", []string{"student"}, -time.Hour)},
			want:   http.StatusUnauthorized,
		},
		{
			name:   "wrong secret → 401",
			cookie: &http.Cookie{Name: "access_token", Value: makeToken(t, wrongSecret, "u1", []string{"student"}, time.Hour)},
			want:   http.StatusUnauthorized,
		},
		{
			name:   "valid token → 200",
			cookie: &http.Cookie{Name: "access_token", Value: makeToken(t, testSecret, "u1", []string{"student"}, time.Hour)},
			want:   http.StatusOK,
		},
	}

	h := RequireAuth(testSecret)(okHandler)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.cookie != nil {
				req.AddCookie(tt.cookie)
			}
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code != tt.want {
				t.Errorf("status = %d, want %d", rr.Code, tt.want)
			}
		})
	}
}

func TestRequireAuth_SetsContext(t *testing.T) {
	var got model.AuthInfo
	var gotOK bool

	h := RequireAuth(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, gotOK = GetAuthInfo(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	tok := makeToken(t, testSecret, "user-42", []string{"admin", "professor"}, time.Hour)
	req := httptest.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: tok})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if !gotOK {
		t.Fatal("expected AuthInfo in context")
	}
	if got.UserID != "user-42" {
		t.Errorf("UserID = %q, want %q", got.UserID, "user-42")
	}
	if got.Email != "test@example.com" {
		t.Errorf("Email = %q, want %q", got.Email, "test@example.com")
	}
	if len(got.Roles) != 2 {
		t.Fatalf("len(Roles) = %d, want 2", len(got.Roles))
	}
	if got.Roles[0] != "admin" || got.Roles[1] != "professor" {
		t.Errorf("Roles = %v, want [admin professor]", got.Roles)
	}
}

// ---------------------------------------------------------------------------
// RequireRole
// ---------------------------------------------------------------------------

func TestRequireRole_WithoutAuth(t *testing.T) {
	h := RequireRole("admin")(okHandler)
	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rr.Code)
	}
}

func TestRequireRole(t *testing.T) {
	tests := []struct {
		name    string
		roles   []string
		allowed []string
		want    int
	}{
		{"student → admin route ⇒ 403", []string{"student"}, []string{"admin"}, http.StatusForbidden},
		{"student → content route ⇒ 403", []string{"student"}, []string{"professor", "admin"}, http.StatusForbidden},
		{"professor → content route ⇒ 200", []string{"professor"}, []string{"professor", "admin"}, http.StatusOK},
		{"admin → content route ⇒ 200", []string{"admin"}, []string{"professor", "admin"}, http.StatusOK},
		{"admin → admin route ⇒ 200", []string{"admin"}, []string{"admin"}, http.StatusOK},
		{"professor → admin route ⇒ 403", []string{"professor"}, []string{"admin"}, http.StatusForbidden},
		{"multi-role user ⇒ 200", []string{"student", "professor"}, []string{"professor"}, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tok := makeToken(t, testSecret, "u1", tt.roles, time.Hour)
			req := httptest.NewRequest("GET", "/test", nil)
			req.AddCookie(&http.Cookie{Name: "access_token", Value: tok})
			rr := httptest.NewRecorder()

			h := Chain(RequireAuth(testSecret), RequireRole(tt.allowed...))(okHandler)
			h.ServeHTTP(rr, req)

			if rr.Code != tt.want {
				t.Errorf("status = %d, want %d", rr.Code, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Error responses must not leak implementation details
// ---------------------------------------------------------------------------

func TestErrorResponses_NoDetailLeak(t *testing.T) {
	sensitiveWords := []string{"cookie", "token", "jwt", "secret", "bearer", "claim", "role", "student", "professor"}

	t.Run("401 body is opaque", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()
		RequireAuth(testSecret)(okHandler).ServeHTTP(rr, req)

		body := strings.ToLower(rr.Body.String())
		for _, word := range sensitiveWords {
			if strings.Contains(body, word) {
				t.Errorf("401 body contains %q: %s", word, rr.Body.String())
			}
		}
		if ct := rr.Header().Get("Content-Type"); ct != "application/problem+json" {
			t.Errorf("Content-Type = %q, want application/problem+json", ct)
		}
	})

	t.Run("403 body is opaque", func(t *testing.T) {
		tok := makeToken(t, testSecret, "u1", []string{"student"}, time.Hour)
		req := httptest.NewRequest("GET", "/test", nil)
		req.AddCookie(&http.Cookie{Name: "access_token", Value: tok})
		rr := httptest.NewRecorder()

		Chain(RequireAuth(testSecret), RequireRole("admin"))(okHandler).ServeHTTP(rr, req)

		body := strings.ToLower(rr.Body.String())
		for _, word := range sensitiveWords {
			if strings.Contains(body, word) {
				t.Errorf("403 body contains %q: %s", word, rr.Body.String())
			}
		}
		if ct := rr.Header().Get("Content-Type"); ct != "application/problem+json" {
			t.Errorf("Content-Type = %q, want application/problem+json", ct)
		}
	})
}
