package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

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
		Error(w, http.StatusUnauthorized, "não autenticado")
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
			Error(w, http.StatusBadRequest, "cursor inválido")
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
		Error(w, http.StatusInternalServerError, "erro ao listar decks")
		return
	}
	JSON(w, http.StatusOK, page)
}

// ListDecksForManagement serves GET /api/content/decks for professor/admin.
// Returns ALL decks including empty and inactive ones for the teach page.
func (h *StudyHandler) ListDecksForManagement(w http.ResponseWriter, r *http.Request) {
	info, ok := middleware.GetAuthInfo(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "não autenticado")
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
			Error(w, http.StatusBadRequest, "cursor inválido")
			return
		}
	}

	page, err := h.svc.ListDecksForManagement(r.Context(), info.UserID, cursorName, cursorID, limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "erro ao listar decks")
		return
	}
	JSON(w, http.StatusOK, page)
}

func (h *StudyHandler) NextCard(w http.ResponseWriter, r *http.Request) {
	info, ok := middleware.GetAuthInfo(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "não autenticado")
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
		Error(w, http.StatusBadRequest, "modo inválido; use: due, random ou wrong")
		return
	}

	topic := r.URL.Query().Get("topic") // optional; empty = all topics

	// Optional comma-separated card UUIDs to exclude from all study modes.
	// Invalid UUIDs are silently dropped to prevent PostgreSQL casting errors.
	var excludeIDs []string
	if raw := r.URL.Query().Get("exclude"); raw != "" {
		for _, id := range strings.Split(raw, ",") {
			id = strings.TrimSpace(id)
			if id != "" && validate.UUID("", id) == nil {
				excludeIDs = append(excludeIDs, id)
			}
		}
	}

	card, err := h.svc.NextCard(r.Context(), info.UserID, deckID, mode, topic, excludeIDs)
	if errors.Is(err, service.ErrForbidden) {
		Error(w, http.StatusForbidden, "acesso negado ao deck")
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		Error(w, http.StatusInternalServerError, "erro ao buscar próximo card")
		return
	}
	JSON(w, http.StatusOK, card)
}

func (h *StudyHandler) SubmitAnswer(w http.ResponseWriter, r *http.Request) {
	info, ok := middleware.GetAuthInfo(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	var req model.AnswerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}

	if err := validate.UUID("card_id", req.CardID); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Result < 0 || req.Result > 2 {
		Error(w, http.StatusBadRequest, "resultado inválido; use 0, 1 ou 2")
		return
	}

	resp, err := h.svc.SubmitAnswer(r.Context(), info.UserID, req.CardID, req.Result)
	if errors.Is(err, service.ErrForbidden) {
		Error(w, http.StatusForbidden, "acesso negado ao deck")
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		Error(w, http.StatusNotFound, "card não encontrado")
		return
	}
	if err != nil {
		Error(w, http.StatusInternalServerError, "erro ao registrar resposta")
		return
	}
	JSON(w, http.StatusOK, resp)
}

func (h *StudyHandler) Progress(w http.ResponseWriter, r *http.Request) {
	info, ok := middleware.GetAuthInfo(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "não autenticado")
		return
	}
	stats, err := h.svc.Progress(r.Context(), info.UserID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "erro ao carregar progresso")
		return
	}
	JSON(w, http.StatusOK, stats)
}

func (h *StudyHandler) Topics(w http.ResponseWriter, r *http.Request) {
	info, ok := middleware.GetAuthInfo(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "não autenticado")
		return
	}
	deckID := r.URL.Query().Get("deckId")
	if err := validate.UUID("deckId", deckID); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	topics, err := h.svc.Topics(r.Context(), info.UserID, deckID)
	if err != nil {
		if errors.Is(err, service.ErrForbidden) {
			Error(w, http.StatusForbidden, "acesso negado ao deck")
			return
		}
		Error(w, http.StatusInternalServerError, "erro ao carregar tópicos")
		return
	}
	JSON(w, http.StatusOK, map[string][]string{"topics": topics})
}

func (h *StudyHandler) Stats(w http.ResponseWriter, r *http.Request) {
	info, ok := middleware.GetAuthInfo(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	deckID := r.URL.Query().Get("deckId")
	if err := validate.UUID("deckId", deckID); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}

	stats, err := h.svc.Stats(r.Context(), info.UserID, deckID)
	if err != nil {
		if errors.Is(err, service.ErrForbidden) {
			Error(w, http.StatusForbidden, "acesso negado ao deck")
			return
		}
		Error(w, http.StatusInternalServerError, "erro ao carregar estatísticas")
		return
	}
	JSON(w, http.StatusOK, stats)
}

// OfflineBundle returns all cards for a deck plus the user's current review
// state, enabling full offline study without further server contact.
// GET /api/study/offline?deckId=...
func (h *StudyHandler) OfflineBundle(w http.ResponseWriter, r *http.Request) {
	info, ok := middleware.GetAuthInfo(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "não autenticado")
		return
	}
	deckID := r.URL.Query().Get("deckId")
	if err := validate.UUID("deckId", deckID); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	bundle, err := h.svc.OfflineBundle(r.Context(), info.UserID, deckID)
	if err != nil {
		if errors.Is(err, service.ErrForbidden) {
			Error(w, http.StatusForbidden, "acesso negado ao deck")
			return
		}
		Error(w, http.StatusInternalServerError, "erro ao carregar dados offline")
		return
	}
	JSON(w, http.StatusOK, bundle)
}

// HideDeck handles POST /api/me/deck-hidden.
// Students can hide or unhide general decks from their home page.
func (h *StudyHandler) HideDeck(w http.ResponseWriter, r *http.Request) {
	info, ok := middleware.GetAuthInfo(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "não autenticado")
		return
	}
	var req model.HideDeckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}
	if err := validate.UUID("deck_id", req.DeckID); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.svc.HideDeck(r.Context(), info.UserID, req.DeckID, req.Hidden); err != nil {
		if errors.Is(err, service.ErrForbidden) {
			Error(w, http.StatusForbidden, "acesso negado ao deck")
			return
		}
		Error(w, http.StatusInternalServerError, "erro ao atualizar visibilidade do deck")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ProfessorStats returns aggregate content and engagement metrics (no PII).
func (h *StudyHandler) ProfessorStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.svc.ProfessorStats(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, "erro ao carregar estatísticas do painel")
		return
	}
	JSON(w, http.StatusOK, stats)
}
