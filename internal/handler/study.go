package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"webapp/internal/middleware"
	"webapp/internal/model"
	"webapp/internal/pagination"
	"webapp/internal/service"
	"webapp/internal/validate"
)

type StudyHandler struct {
	svc *service.StudyService
}

func NewStudyHandler(svc *service.StudyService) *StudyHandler {
	return &StudyHandler{svc: svc}
}

func (h *StudyHandler) ListDecks(w http.ResponseWriter, r *http.Request) {
	info, ok := middleware.GetAuthInfo(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	limit, err := pagination.ParseLimit(r, pagination.DefaultLimit, pagination.MaxLimit)
	if err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}

	var cursorName, cursorID string
	if c := r.URL.Query().Get("cursor"); c != "" {
		cursorName, cursorID, err = pagination.DecodeNameIDCursor(c)
		if err != nil {
			Error(w, http.StatusBadRequest, "invalid cursor")
			return
		}
	}

	// Professors and admins see all decks (including inactive/expired) for management.
	// Students only see active, non-expired decks.
	roles := info.Roles
	showAll := false
	for _, role := range roles {
		if role == "professor" || role == "admin" {
			showAll = true
			break
		}
	}

	page, err := h.svc.ListDecks(r.Context(), info.UserID, cursorName, cursorID, limit, showAll)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to list decks")
		return
	}
	JSON(w, http.StatusOK, page)
}

// ListDecksForManagement serves GET /api/content/decks for professor/admin.
// Returns ALL decks including empty and inactive ones for the teach page.
func (h *StudyHandler) ListDecksForManagement(w http.ResponseWriter, r *http.Request) {
	info, ok := middleware.GetAuthInfo(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	limit, err := pagination.ParseLimit(r, pagination.DefaultLimit, pagination.MaxLimit)
	if err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}

	var cursorName, cursorID string
	if c := r.URL.Query().Get("cursor"); c != "" {
		cursorName, cursorID, err = pagination.DecodeNameIDCursor(c)
		if err != nil {
			Error(w, http.StatusBadRequest, "invalid cursor")
			return
		}
	}

	page, err := h.svc.ListDecksForManagement(r.Context(), info.UserID, cursorName, cursorID, limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to list decks")
		return
	}
	JSON(w, http.StatusOK, page)
}

func (h *StudyHandler) NextCard(w http.ResponseWriter, r *http.Request) {
	info, ok := middleware.GetAuthInfo(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	deckID := r.URL.Query().Get("deckId")
	if err := validate.UUID("deckId", deckID); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}

	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "due"
	}
	if mode != "due" && mode != "random" && mode != "wrong" {
		Error(w, http.StatusBadRequest, "mode must be due, random, or wrong")
		return
	}

	topic := r.URL.Query().Get("topic") // optional; empty = all topics

	card, err := h.svc.NextCard(r.Context(), info.UserID, deckID, mode, topic)
	if errors.Is(err, sql.ErrNoRows) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to get next card")
		return
	}
	JSON(w, http.StatusOK, card)
}

func (h *StudyHandler) SubmitAnswer(w http.ResponseWriter, r *http.Request) {
	info, ok := middleware.GetAuthInfo(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var req model.AnswerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := validate.UUID("card_id", req.CardID); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Result < 0 || req.Result > 2 {
		Error(w, http.StatusBadRequest, "result must be 0, 1, or 2")
		return
	}

	resp, err := h.svc.SubmitAnswer(r.Context(), info.UserID, req.CardID, req.Result)
	if errors.Is(err, sql.ErrNoRows) {
		Error(w, http.StatusNotFound, "card not found")
		return
	}
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to record answer")
		return
	}
	JSON(w, http.StatusOK, resp)
}

func (h *StudyHandler) Progress(w http.ResponseWriter, r *http.Request) {
	info, ok := middleware.GetAuthInfo(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	stats, err := h.svc.Progress(r.Context(), info.UserID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to load progress")
		return
	}
	JSON(w, http.StatusOK, stats)
}

func (h *StudyHandler) Topics(w http.ResponseWriter, r *http.Request) {
	if _, ok := middleware.GetAuthInfo(r.Context()); !ok {
		Error(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	deckID := r.URL.Query().Get("deckId")
	if err := validate.UUID("deckId", deckID); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	topics, err := h.svc.Topics(r.Context(), deckID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to load topics")
		return
	}
	JSON(w, http.StatusOK, map[string][]string{"topics": topics})
}

func (h *StudyHandler) Stats(w http.ResponseWriter, r *http.Request) {
	info, ok := middleware.GetAuthInfo(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	deckID := r.URL.Query().Get("deckId")
	if err := validate.UUID("deckId", deckID); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}

	stats, err := h.svc.Stats(r.Context(), info.UserID, deckID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to load stats")
		return
	}
	JSON(w, http.StatusOK, stats)
}

// ProfessorStats returns aggregate content and engagement metrics (no PII).
func (h *StudyHandler) ProfessorStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.svc.ProfessorStats(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to load professor stats")
		return
	}
	JSON(w, http.StatusOK, stats)
}
