package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"webapp/internal/service"
)

type PushHandler struct {
	svc *service.PushService
}

func NewPushHandler(svc *service.PushService) *PushHandler {
	return &PushHandler{svc: svc}
}

// GET /api/push/key — returns the VAPID public key.
// Browsers need this to create a push subscription associated with this server.
func (h *PushHandler) PublicKey(w http.ResponseWriter, r *http.Request) {
	key := h.svc.PublicKey()
	if key == "" {
		Error(w, http.StatusServiceUnavailable, "notificações push não configuradas")
		return
	}
	JSON(w, http.StatusOK, map[string]string{"public_key": key})
}

// POST /api/push/subscribe — save a push subscription for the calling user.
// Body: { "endpoint": "...", "keys": { "p256dh": "...", "auth": "..." } }
func (h *PushHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Endpoint string `json:"endpoint"`
		Keys     struct {
			P256DH string `json:"p256dh"`
			Auth   string `json:"auth"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}
	req.Endpoint = strings.TrimSpace(req.Endpoint)
	if req.Endpoint == "" || req.Keys.P256DH == "" || req.Keys.Auth == "" {
		Error(w, http.StatusBadRequest, "endpoint, keys.p256dh e keys.auth são obrigatórios")
		return
	}
	if len(req.Endpoint) > 2048 {
		Error(w, http.StatusBadRequest, "endpoint inválido")
		return
	}

	if err := h.svc.Subscribe(r.Context(), req.Endpoint, req.Keys.P256DH, req.Keys.Auth); err != nil {
		Error(w, http.StatusInternalServerError, "erro ao salvar inscrição")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/push/subscribe — remove a push subscription.
// Body: { "endpoint": "..." }
func (h *PushHandler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Endpoint string `json:"endpoint"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}
	if strings.TrimSpace(req.Endpoint) == "" {
		Error(w, http.StatusBadRequest, "endpoint é obrigatório")
		return
	}
	if err := h.svc.Unsubscribe(r.Context(), req.Endpoint); err != nil {
		Error(w, http.StatusInternalServerError, "erro ao remover inscrição")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
