package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"webapp/internal/middleware"
	"webapp/internal/model"
)

func authCtx(r *http.Request) *http.Request {
	ctx := middleware.WithAuthInfo(r.Context(), model.AuthInfo{
		UserID: "00000000-0000-0000-0000-000000000001",
		Email:  "test@example.com",
		Roles:  []string{"student"},
	})
	return r.WithContext(ctx)
}

// A nil-service handler is safe for validation tests because all tested
// code paths return before the service is reached.
var studyH = &StudyHandler{}

func TestNextCard_Validation(t *testing.T) {
	tests := []struct {
		name string
		url  string
		auth bool
		want int
	}{
		{"no auth → 401", "/api/study/next?deckId=abc", false, http.StatusUnauthorized},
		{"missing deckId → 400", "/api/study/next", true, http.StatusBadRequest},
		{"invalid deckId → 400", "/api/study/next?deckId=not-a-uuid", true, http.StatusBadRequest},
		{"invalid mode → 400", "/api/study/next?deckId=00000000-0000-0000-0000-000000000001&mode=invalid", true, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			if tt.auth {
				req = authCtx(req)
			}
			rr := httptest.NewRecorder()
			studyH.NextCard(rr, req)
			if rr.Code != tt.want {
				t.Errorf("status = %d, want %d; body: %s", rr.Code, tt.want, rr.Body.String())
			}
		})
	}
}

func TestSubmitAnswer_Validation(t *testing.T) {
	tests := []struct {
		name string
		body string
		auth bool
		want int
	}{
		{"no auth → 401", `{}`, false, http.StatusUnauthorized},
		{"empty body → 400", ``, true, http.StatusBadRequest},
		{"invalid JSON → 400", `{bad`, true, http.StatusBadRequest},
		{"invalid card_id → 400", `{"card_id":"nope","result":1}`, true, http.StatusBadRequest},
		{"result too high → 400", `{"card_id":"00000000-0000-0000-0000-000000000001","result":3}`, true, http.StatusBadRequest},
		{"result negative → 400", `{"card_id":"00000000-0000-0000-0000-000000000001","result":-1}`, true, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/study/answer", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			if tt.auth {
				req = authCtx(req)
			}
			rr := httptest.NewRecorder()
			studyH.SubmitAnswer(rr, req)
			if rr.Code != tt.want {
				t.Errorf("status = %d, want %d; body: %s", rr.Code, tt.want, rr.Body.String())
			}
		})
	}
}

func TestStats_Validation(t *testing.T) {
	tests := []struct {
		name string
		url  string
		auth bool
		want int
	}{
		{"no auth → 401", "/api/stats?deckId=abc", false, http.StatusUnauthorized},
		{"missing deckId → 400", "/api/stats", true, http.StatusBadRequest},
		{"invalid deckId → 400", "/api/stats?deckId=xyz", true, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			if tt.auth {
				req = authCtx(req)
			}
			rr := httptest.NewRecorder()
			studyH.Stats(rr, req)
			if rr.Code != tt.want {
				t.Errorf("status = %d, want %d; body: %s", rr.Code, tt.want, rr.Body.String())
			}
		})
	}
}
