package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"webapp/internal/service"
	"webapp/internal/validate"
)

type ClassHandler struct {
	svc *service.ClassService
}

func NewClassHandler(svc *service.ClassService) *ClassHandler {
	return &ClassHandler{svc: svc}
}

// GET /api/classes
func (h *ClassHandler) ListMyClasses(w http.ResponseWriter, r *http.Request) {
	classes, err := h.svc.ListMyClasses(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to list classes")
		return
	}
	JSON(w, http.StatusOK, map[string]any{"items": classes})
}

// POST /api/classes
func (h *ClassHandler) CreateClass(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string  `json:"name"`
		Description *string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	name := strings.TrimSpace(req.Name)
	if err := validate.Required("name", name); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := validate.StringField("name", name, 120); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	cl, err := h.svc.CreateClass(r.Context(), name, req.Description)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to create class")
		return
	}
	JSON(w, http.StatusCreated, cl)
}

// GET /api/classes/{id}
func (h *ClassHandler) GetClass(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := validate.UUID("id", id); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	cl, err := h.svc.GetClass(r.Context(), id)
	if err != nil {
		h.classError(w, err)
		return
	}
	JSON(w, http.StatusOK, cl)
}

// PUT /api/classes/{id}
func (h *ClassHandler) UpdateClass(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := validate.UUID("id", id); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	var req struct {
		Name        string  `json:"name"`
		Description *string `json:"description"`
		IsActive    bool    `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	name := strings.TrimSpace(req.Name)
	if err := validate.Required("name", name); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	cl, err := h.svc.UpdateClass(r.Context(), id, name, req.Description, req.IsActive)
	if err != nil {
		h.classError(w, err)
		return
	}
	JSON(w, http.StatusOK, cl)
}

// DELETE /api/classes/{id}
func (h *ClassHandler) DeleteClass(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := validate.UUID("id", id); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.svc.DeleteClass(r.Context(), id); err != nil {
		h.classError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/classes/{id}/invite  — regenerate invite code
func (h *ClassHandler) RegenerateInviteCode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := validate.UUID("id", id); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	code, err := h.svc.RegenerateInviteCode(r.Context(), id)
	if err != nil {
		h.classError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]string{"invite_code": code})
}

// POST /api/classes/join
func (h *ClassHandler) JoinClass(w http.ResponseWriter, r *http.Request) {
	var req struct {
		InviteCode string `json:"invite_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.InviteCode) == "" {
		Error(w, http.StatusBadRequest, "invite_code is required")
		return
	}
	cl, err := h.svc.JoinClass(r.Context(), req.InviteCode)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrClassNotFound):
			Error(w, http.StatusNotFound, "código de turma não encontrado. Verifique com seu professor.")
		case errors.Is(err, service.ErrClassInactive):
			Error(w, http.StatusConflict, "esta turma não está mais ativa.")
		case errors.Is(err, service.ErrAlreadyMember):
			Error(w, http.StatusConflict, "você já está inscrito nesta turma.")
		default:
			Error(w, http.StatusInternalServerError, "failed to join class")
		}
		return
	}
	JSON(w, http.StatusOK, cl)
}

// DELETE /api/classes/{id}/leave
func (h *ClassHandler) LeaveClass(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := validate.UUID("id", id); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.svc.LeaveClass(r.Context(), id); err != nil {
		h.classError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/classes/{id}/decks
func (h *ClassHandler) AssignDeck(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := validate.UUID("id", id); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	var req struct {
		DeckID string `json:"deck_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.UUID("deck_id", req.DeckID); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.svc.AssignDeck(r.Context(), id, req.DeckID); err != nil {
		h.classError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/classes/{id}/decks/{deckId}
func (h *ClassHandler) UnassignDeck(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	deckID := r.PathValue("deckId")
	if err := validate.UUID("id", id); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validate.UUID("deckId", deckID); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.svc.UnassignDeck(r.Context(), id, deckID); err != nil {
		h.classError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/classes/{id}/decks
func (h *ClassHandler) ListClassDecks(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := validate.UUID("id", id); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	decks, err := h.svc.ListClassDecks(r.Context(), id)
	if err != nil {
		h.classError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"items": decks})
}

// GET /api/classes/{id}/stats
func (h *ClassHandler) ClassStats(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := validate.UUID("id", id); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	stats, err := h.svc.GetClassStats(r.Context(), id)
	if err != nil {
		h.classError(w, err)
		return
	}
	JSON(w, http.StatusOK, stats)
}

// GET /api/classes/overview — compact summary of all professor's classes
func (h *ClassHandler) ClassOverview(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.GetClassOverview(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to load class overview")
		return
	}
	JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *ClassHandler) classError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrClassNotFound):
		Error(w, http.StatusNotFound, "turma não encontrada")
	case errors.Is(err, service.ErrForbidden):
		Error(w, http.StatusForbidden, "você não tem permissão para modificar esta turma")
	default:
		Error(w, http.StatusInternalServerError, "class operation failed")
	}
}
