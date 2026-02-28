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
		Error(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	user, err := h.users.FindByID(r.Context(), info.UserID)
	if err != nil {
		Error(w, http.StatusNotFound, "user not found")
		return
	}

	roles, err := h.users.RolesByUserID(r.Context(), info.UserID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to load roles")
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
