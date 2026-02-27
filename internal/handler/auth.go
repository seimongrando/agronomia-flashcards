package handler

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"

	"golang.org/x/oauth2"

	"webapp/internal/model"
	"webapp/internal/service"
)

type AuthHandler struct {
	oauth   *oauth2.Config
	authSvc *service.AuthService
	secure  bool
}

func NewAuthHandler(oauth *oauth2.Config, authSvc *service.AuthService, secure bool) *AuthHandler {
	return &AuthHandler{oauth: oauth, authSvc: authSvc, secure: secure}
}

// GoogleRedirect starts the OAuth flow by redirecting the user to Google's consent screen.
func (h *AuthHandler) GoogleRedirect(w http.ResponseWriter, r *http.Request) {
	state := randomState()
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/auth/google/callback",
		MaxAge:   300,
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, h.oauth.AuthCodeURL(state), http.StatusTemporaryRedirect)
}

// GoogleCallback exchanges the authorization code for user info, upserts the
// user, assigns roles, creates a JWT and sets it as an HttpOnly cookie.
func (h *AuthHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil || stateCookie.Value == "" || stateCookie.Value != r.URL.Query().Get("state") {
		http.Redirect(w, r, "/?error=invalid_state", http.StatusTemporaryRedirect)
		return
	}

	// Clear one-time state cookie immediately.
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    "",
		Path:     "/auth/google/callback",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.secure,
	})

	if errParam := r.URL.Query().Get("error"); errParam != "" {
		http.Redirect(w, r, "/?error=oauth_denied", http.StatusTemporaryRedirect)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Redirect(w, r, "/?error=missing_code", http.StatusTemporaryRedirect)
		return
	}

	token, err := h.oauth.Exchange(r.Context(), code)
	if err != nil {
		slog.Error("oauth code exchange failed", "error", err)
		http.Redirect(w, r, "/?error=exchange_failed", http.StatusTemporaryRedirect)
		return
	}

	profile, err := h.fetchProfile(r, token)
	if err != nil {
		slog.Error("failed to fetch google profile", "error", err)
		http.Redirect(w, r, "/?error=profile_failed", http.StatusTemporaryRedirect)
		return
	}

	jwtToken, err := h.authSvc.LoginWithGoogle(r.Context(), profile)
	if err != nil {
		slog.Error("login with google failed", "error", err)
		http.Redirect(w, r, "/?error=login_failed", http.StatusTemporaryRedirect)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    jwtToken,
		Path:     "/",
		MaxAge:   int(h.authSvc.TokenExpiry().Seconds()),
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

// Logout clears the access_token cookie.
func (h *AuthHandler) Logout(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
	})
	JSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

func (h *AuthHandler) fetchProfile(r *http.Request, token *oauth2.Token) (model.GoogleProfile, error) {
	client := h.oauth.Client(r.Context(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		return model.GoogleProfile{}, err
	}
	defer resp.Body.Close()

	var profile model.GoogleProfile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return model.GoogleProfile{}, err
	}
	return profile, nil
}

func randomState() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}
