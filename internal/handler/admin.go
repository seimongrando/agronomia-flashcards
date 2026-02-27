package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"webapp/internal/middleware"
	"webapp/internal/model"
	"webapp/internal/pagination"
	"webapp/internal/service"
	"webapp/internal/validate"
)

type AdminHandler struct {
	svc *service.AdminService
}

func NewAdminHandler(svc *service.AdminService) *AdminHandler {
	return &AdminHandler{svc: svc}
}

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := middleware.GetAuthInfo(r.Context()); !ok {
		Error(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	// Accept both ?q= (new) and ?query= (legacy) for backward compatibility.
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		q = strings.TrimSpace(r.URL.Query().Get("query"))
	}

	limit, err := pagination.ParseLimit(r, pagination.DefaultLimit, pagination.MaxLimit)
	if err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}

	var cursorTS time.Time
	var cursorID string
	if c := r.URL.Query().Get("cursor"); c != "" {
		cursorTS, cursorID, err = pagination.DecodeTimestampIDCursor(c)
		if err != nil {
			Error(w, http.StatusBadRequest, "invalid cursor")
			return
		}
	}

	page, err := h.svc.ListUsers(r.Context(), q, cursorTS, cursorID, limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	JSON(w, http.StatusOK, page)
}

func (h *AdminHandler) SetRoles(w http.ResponseWriter, r *http.Request) {
	info, ok := middleware.GetAuthInfo(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	targetID := r.PathValue("id")
	if err := validate.UUID("id", targetID); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}

	var req model.SetRolesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Add) == 0 && len(req.Remove) == 0 {
		Error(w, http.StatusBadRequest, "at least one of add or remove is required")
		return
	}

	roles, err := h.svc.SetRoles(r.Context(), info.UserID, targetID, req.Add, req.Remove)
	if err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "not found"):
			Error(w, http.StatusNotFound, "user not found")
		case strings.Contains(msg, "invalid role"):
			Error(w, http.StatusBadRequest, msg)
		case strings.Contains(msg, "must retain"):
			Error(w, http.StatusConflict, msg)
		default:
			Error(w, http.StatusInternalServerError, "failed to update roles")
		}
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"user_id": targetID,
		"roles":   roles,
	})
}
