package handler

import (
	"net/http"

	"webapp/internal/middleware"
	"webapp/internal/model"
	"webapp/internal/repository"
)

type MeHandler struct {
	users *repository.UserRepo
}

func NewMeHandler(users *repository.UserRepo) *MeHandler {
	return &MeHandler{users: users}
}

// Me returns the authenticated user's public profile and roles.
// The response uses MeResponse — a minimal DTO that never exposes google_sub,
// internal timestamps, or other fields not required by the frontend (LGPD/minimisation).
func (h *MeHandler) Me(w http.ResponseWriter, r *http.Request) {
	info, ok := middleware.GetAuthInfo(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	user, err := h.users.FindByID(r.Context(), info.UserID)
	if err != nil {
		Error(w, http.StatusNotFound, "usuário não encontrado")
		return
	}

	roles, err := h.users.RolesByUserID(r.Context(), info.UserID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "erro ao carregar perfil")
		return
	}

	roleStrs := make([]string, len(roles))
	for i, role := range roles {
		roleStrs[i] = string(role)
	}

	JSON(w, http.StatusOK, map[string]any{
		"user": model.MeResponse{
			ID:         user.ID,
			Name:       user.Name,
			Email:      user.Email,
			PictureURL: user.PictureURL,
		},
		"roles": roleStrs,
	})
}
