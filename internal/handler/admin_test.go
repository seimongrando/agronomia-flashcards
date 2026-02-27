package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"webapp/internal/middleware"
	"webapp/internal/model"
)

func adminCtx(r *http.Request) *http.Request {
	ctx := middleware.WithAuthInfo(r.Context(), model.AuthInfo{
		UserID: "00000000-0000-0000-0000-000000000099",
		Email:  "admin@example.com",
		Roles:  []string{"admin"},
	})
	return r.WithContext(ctx)
}

func studentCtx(r *http.Request) *http.Request {
	ctx := middleware.WithAuthInfo(r.Context(), model.AuthInfo{
		UserID: "00000000-0000-0000-0000-000000000002",
		Email:  "student@example.com",
		Roles:  []string{"student"},
	})
	return r.WithContext(ctx)
}

func professorCtx(r *http.Request) *http.Request {
	ctx := middleware.WithAuthInfo(r.Context(), model.AuthInfo{
		UserID: "00000000-0000-0000-0000-000000000003",
		Email:  "prof@example.com",
		Roles:  []string{"professor"},
	})
	return r.WithContext(ctx)
}

// adminH uses nil service — all tested paths return before the service is reached.
var adminH = &AdminHandler{}

func TestListUsers_Authorization(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*http.Request) *http.Request
		want  int
	}{
		{"no auth → 401", func(r *http.Request) *http.Request { return r }, http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/admin/users", nil)
			req = tt.setup(req)
			rr := httptest.NewRecorder()
			adminH.ListUsers(rr, req)
			if rr.Code != tt.want {
				t.Errorf("status = %d, want %d; body: %s", rr.Code, tt.want, rr.Body.String())
			}
		})
	}
}

func TestSetRoles_Authorization(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*http.Request) *http.Request
		want  int
	}{
		{"no auth → 401", func(r *http.Request) *http.Request { return r }, http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/admin/users/00000000-0000-0000-0000-000000000001/roles",
				strings.NewReader(`{"add":["professor"]}`))
			req.Header.Set("Content-Type", "application/json")
			req = tt.setup(req)
			rr := httptest.NewRecorder()
			adminH.SetRoles(rr, req)
			if rr.Code != tt.want {
				t.Errorf("status = %d, want %d; body: %s", rr.Code, tt.want, rr.Body.String())
			}
		})
	}
}

func TestSetRoles_Validation(t *testing.T) {
	tests := []struct {
		name string
		url  string
		body string
		want int
	}{
		{
			"invalid target id → 400",
			"/api/admin/users/not-a-uuid/roles",
			`{"add":["professor"]}`,
			http.StatusBadRequest,
		},
		{
			"empty body → 400",
			"/api/admin/users/00000000-0000-0000-0000-000000000001/roles",
			``,
			http.StatusBadRequest,
		},
		{
			"invalid JSON → 400",
			"/api/admin/users/00000000-0000-0000-0000-000000000001/roles",
			`{bad`,
			http.StatusBadRequest,
		},
		{
			"no add or remove → 400",
			"/api/admin/users/00000000-0000-0000-0000-000000000001/roles",
			`{}`,
			http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", tt.url, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req.SetPathValue("id", extractPathID(tt.url))
			req = adminCtx(req)
			rr := httptest.NewRecorder()
			adminH.SetRoles(rr, req)
			if rr.Code != tt.want {
				t.Errorf("status = %d, want %d; body: %s", rr.Code, tt.want, rr.Body.String())
			}
		})
	}
}

// TestRequireRole_Middleware tests that RequireRole blocks non-admin users
// at the middleware layer (403), which is how the routes are wired in main.go.
func TestRequireRole_AdminOnly(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	chain := middleware.Chain(
		middleware.RequireRole("admin"),
	)
	guarded := chain(inner)

	tests := []struct {
		name  string
		setup func(*http.Request) *http.Request
		want  int
	}{
		{"student → 403", studentCtx, http.StatusForbidden},
		{"professor → 403", professorCtx, http.StatusForbidden},
		{"admin → 200", adminCtx, http.StatusOK},
		{"no auth → 401", func(r *http.Request) *http.Request { return r }, http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/admin/users", nil)
			req = tt.setup(req)
			rr := httptest.NewRecorder()
			guarded.ServeHTTP(rr, req)
			if rr.Code != tt.want {
				t.Errorf("status = %d, want %d; body: %s", rr.Code, tt.want, rr.Body.String())
			}
		})
	}
}

func extractPathID(url string) string {
	parts := strings.Split(url, "/")
	for i, p := range parts {
		if p == "users" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}
