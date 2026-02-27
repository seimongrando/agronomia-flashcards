package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

var csrfOKHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
})

func csrfRequest(method, path, origin, referer string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	return req
}

// ── Non-mutating methods are always allowed ───────────────────────────────────

func TestCSRF_GetAlwaysAllowed(t *testing.T) {
	h := CSRF(nil, false)(csrfOKHandler)
	for _, m := range []string{"GET", "HEAD", "OPTIONS"} {
		req := csrfRequest(m, "/api/decks", "", "")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", m, rr.Code)
		}
	}
}

// ── OAuth redirect paths are exempt ──────────────────────────────────────────

func TestCSRF_OAuthExempt(t *testing.T) {
	h := CSRF(nil, false)(csrfOKHandler)
	for _, path := range []string{"/auth/google", "/auth/google/callback"} {
		req := csrfRequest("POST", path, "", "")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("POST %s: status = %d, want 200 (exempt)", path, rr.Code)
		}
	}
}

// ── Dev mode: missing origin allowed ─────────────────────────────────────────

func TestCSRF_DevModeAllowsMissingOrigin(t *testing.T) {
	h := CSRF(nil, true)(csrfOKHandler) // isDev=true
	req := csrfRequest("POST", "/api/study/answer", "", "")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("dev mode, no origin: status = %d, want 200", rr.Code)
	}
}

// ── Production mode: missing origin rejected ──────────────────────────────────

func TestCSRF_ProdModeRejectsMissingOrigin(t *testing.T) {
	h := CSRF(nil, false)(csrfOKHandler) // isDev=false
	req := csrfRequest("POST", "/api/study/answer", "", "")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("prod mode, no origin: status = %d, want 403", rr.Code)
	}
}

// ── Explicit allowedOrigins list ──────────────────────────────────────────────

func TestCSRF_AllowedOriginAccepted(t *testing.T) {
	allowed := []string{"https://app.example.com"}
	h := CSRF(allowed, false)(csrfOKHandler)

	req := csrfRequest("POST", "/api/content/cards", "https://app.example.com", "")
	req.Host = "app.example.com"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("allowed origin: status = %d, want 200", rr.Code)
	}
}

func TestCSRF_ForeignOriginRejected(t *testing.T) {
	allowed := []string{"https://app.example.com"}
	h := CSRF(allowed, false)(csrfOKHandler)

	req := csrfRequest("DELETE", "/api/content/cards/id", "https://evil.example.com", "")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("foreign origin: status = %d, want 403", rr.Code)
	}
}

// ── No allowedOrigins: same-site match ───────────────────────────────────────

func TestCSRF_SameSiteMatchedByHost(t *testing.T) {
	h := CSRF(nil, false)(csrfOKHandler)
	req := csrfRequest("POST", "/api/study/answer", "http://localhost:8080", "")
	req.Host = "localhost:8080"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("same-site host: status = %d, want 200", rr.Code)
	}
}

func TestCSRF_DifferentHostRejected(t *testing.T) {
	h := CSRF(nil, false)(csrfOKHandler)
	req := csrfRequest("PUT", "/api/content/decks/id", "http://attacker.io", "")
	req.Host = "myapp.com"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("different host: status = %d, want 403", rr.Code)
	}
}

// ── Referer fallback ──────────────────────────────────────────────────────────

func TestCSRF_RefererFallback(t *testing.T) {
	h := CSRF(nil, false)(csrfOKHandler)
	// Origin absent; Referer present and matches host
	req := csrfRequest("POST", "/api/study/answer", "", "http://localhost:8080/study.html")
	req.Host = "localhost:8080"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("referer fallback: status = %d, want 200", rr.Code)
	}
}

func TestCSRF_RefererMismatch(t *testing.T) {
	h := CSRF(nil, false)(csrfOKHandler)
	req := csrfRequest("POST", "/api/content/cards", "", "https://evil.io/page")
	req.Host = "myapp.com"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("referer mismatch: status = %d, want 403", rr.Code)
	}
}

// ── Response format ───────────────────────────────────────────────────────────

func TestCSRF_ResponseContentType(t *testing.T) {
	h := CSRF(nil, false)(csrfOKHandler)
	req := csrfRequest("POST", "/api/content/cards", "https://bad.io", "")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if ct := rr.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", ct)
	}
	if rr.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rr.Code)
	}
}
