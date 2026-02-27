package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"webapp/internal/csvparse"
	"webapp/internal/middleware"
	"webapp/internal/model"
	"webapp/internal/pagination"
	"webapp/internal/service"
	"webapp/internal/validate"
)

type ContentHandler struct {
	svc *service.ContentService
}

func NewContentHandler(svc *service.ContentService) *ContentHandler {
	return &ContentHandler{svc: svc}
}

// --- Deck endpoints ---

func (h *ContentHandler) CreateDeck(w http.ResponseWriter, r *http.Request) {
	var req model.CreateDeckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.Required("name", req.Name); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	name, err := validate.StringField("name", req.Name, model.MaxDeckNameLen)
	if err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}

	deck, err := h.svc.CreateDeck(r.Context(), name, req.Description)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to create deck")
		return
	}
	JSON(w, http.StatusCreated, deck)
}

func (h *ContentHandler) GetDeck(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := validate.UUID("id", id); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	deck, err := h.svc.GetDeck(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		Error(w, http.StatusNotFound, "deck not found")
		return
	}
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to get deck")
		return
	}
	JSON(w, http.StatusOK, deck)
}

func (h *ContentHandler) UpdateDeck(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := validate.UUID("id", id); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}

	var req model.UpdateDeckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.Required("name", req.Name); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	name, err := validate.StringField("name", req.Name, model.MaxDeckNameLen)
	if err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}

	deck, err := h.svc.UpdateDeck(r.Context(), id, name, req.Description)
	if errors.Is(err, sql.ErrNoRows) {
		Error(w, http.StatusNotFound, "deck not found")
		return
	}
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to update deck")
		return
	}
	JSON(w, http.StatusOK, deck)
}

func (h *ContentHandler) DeleteDeck(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := validate.UUID("id", id); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}

	err := h.svc.DeleteDeck(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		Error(w, http.StatusNotFound, "deck not found")
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "card(s)") {
			Error(w, http.StatusConflict, err.Error())
			return
		}
		Error(w, http.StatusInternalServerError, "failed to delete deck")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Card endpoints ---

// ListCards returns a paginated, search-filtered list of cards for a deck.
// Answer is excluded from the list payload; use GetCardDetail for the full card.
func (h *ContentHandler) ListCards(w http.ResponseWriter, r *http.Request) {
	deckID := r.URL.Query().Get("deckId")
	if err := validate.UUID("deckId", deckID); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}

	limit, err := pagination.ParseLimit(r, pagination.DefaultLimit, pagination.MaxLimit)
	if err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}

	searchQuery := strings.TrimSpace(r.URL.Query().Get("q"))

	var cursorTS time.Time
	var cursorID string
	if c := r.URL.Query().Get("cursor"); c != "" {
		cursorTS, cursorID, err = pagination.DecodeTimestampIDCursor(c)
		if err != nil {
			Error(w, http.StatusBadRequest, "invalid cursor")
			return
		}
	}

	page, err := h.svc.ListCards(r.Context(), deckID, searchQuery, cursorTS, cursorID, limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to list cards")
		return
	}
	JSON(w, http.StatusOK, page)
}

// GetCardDetail returns the full card (including answer) for editing.
func (h *ContentHandler) GetCardDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := validate.UUID("id", id); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	card, err := h.svc.GetCard(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		Error(w, http.StatusNotFound, "card not found")
		return
	}
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to get card")
		return
	}
	JSON(w, http.StatusOK, card)
}

func (h *ContentHandler) CreateCard(w http.ResponseWriter, r *http.Request) {
	var req model.CreateCardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.UUID("deck_id", req.DeckID); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	ct := model.CardType(req.Type)
	if !ct.Valid() {
		Error(w, http.StatusBadRequest, "type must be conceito, processo, aplicacao, or comparacao")
		return
	}
	if err := validate.Required("question", req.Question); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	question, err := validate.StringField("question", req.Question, model.MaxQuestionLen)
	if err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validate.Required("answer", req.Answer); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	answer, err := validate.StringField("answer", req.Answer, model.MaxAnswerLen)
	if err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	topic, source, err := validateOptionalFields(req.Topic, req.Source)
	if err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}

	card := model.Card{
		DeckID:   req.DeckID,
		Topic:    topic,
		Type:     ct,
		Question: question,
		Answer:   answer,
		Source:   source,
	}

	created, err := h.svc.CreateCard(r.Context(), card)
	if err != nil {
		if strings.Contains(err.Error(), "deck not found") {
			Error(w, http.StatusNotFound, "deck not found")
			return
		}
		Error(w, http.StatusInternalServerError, "failed to create card")
		return
	}
	JSON(w, http.StatusCreated, created)
}

func (h *ContentHandler) UpdateCard(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := validate.UUID("id", id); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}

	var req model.UpdateCardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ct := model.CardType(req.Type)
	if !ct.Valid() {
		Error(w, http.StatusBadRequest, "type must be conceito, processo, aplicacao, or comparacao")
		return
	}
	if err := validate.Required("question", req.Question); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	question, err := validate.StringField("question", req.Question, model.MaxQuestionLen)
	if err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validate.Required("answer", req.Answer); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	answer, err := validate.StringField("answer", req.Answer, model.MaxAnswerLen)
	if err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	topic, source, err := validateOptionalFields(req.Topic, req.Source)
	if err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}

	card := model.Card{
		ID:       id,
		Topic:    topic,
		Type:     ct,
		Question: question,
		Answer:   answer,
		Source:   source,
	}

	if err := h.svc.UpdateCard(r.Context(), card); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			Error(w, http.StatusNotFound, "card not found")
			return
		}
		Error(w, http.StatusInternalServerError, "failed to update card")
		return
	}
	JSON(w, http.StatusOK, card)
}

func (h *ContentHandler) DeleteCard(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := validate.UUID("id", id); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.DeleteCard(r.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			Error(w, http.StatusNotFound, "card not found")
			return
		}
		Error(w, http.StatusInternalServerError, "failed to delete card")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- CSV upload ---

// dryRunResponse is the shape returned for ?dryRun=1 requests.
// It combines the row-level preview with estimated insert/update counts so the
// UI can show exactly what would happen before committing.
type dryRunResponse struct {
	Rows         []csvparse.Row `json:"rows"`
	TotalRows    int            `json:"total_rows"`
	ValidRows    int            `json:"valid_rows"`
	InvalidCount int            `json:"invalid_count"`
	WouldInsert  int            `json:"would_insert"`
	WouldUpdate  int            `json:"would_update"`
}

func (h *ContentHandler) UploadCSV(w http.ResponseWriter, r *http.Request) {
	info, ok := middleware.GetAuthInfo(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	// Optional deckId for single-deck mode (deck column may be absent in CSV).
	var defaultDeckID, defaultDeckName string
	if dkID := r.URL.Query().Get("deckId"); dkID != "" {
		if err := validate.UUID("deckId", dkID); err != nil {
			Error(w, http.StatusBadRequest, err.Error())
			return
		}
		dk, err := h.svc.GetDeck(r.Context(), dkID)
		if err != nil {
			Error(w, http.StatusNotFound, "deck not found")
			return
		}
		defaultDeckID = dk.ID
		defaultDeckName = dk.Name
	}

	r.Body = http.MaxBytesReader(w, r.Body, csvparse.MaxFileSize)
	if err := r.ParseMultipartForm(csvparse.MaxFileSize); err != nil {
		Error(w, http.StatusBadRequest, "file too large or invalid multipart form (max 2 MB)")
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			r.MultipartForm.RemoveAll()
		}
	}()

	file, header, err := r.FormFile("file")
	if err != nil {
		Error(w, http.StatusBadRequest, "file field is required")
		return
	}
	defer file.Close()

	opts := csvparse.ParseOptions{DefaultDeck: defaultDeckName}
	parsed, err := csvparse.Parse(file, opts)
	if err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}

	dryRun := r.URL.Query().Get("dryRun") != "0"

	if dryRun {
		wouldInsert, wouldUpdate, _ := h.svc.EstimateDryRun(r.Context(), parsed, defaultDeckID)

		rows := parsed.Rows
		if len(rows) > csvparse.MaxPreviewRows {
			rows = rows[:csvparse.MaxPreviewRows]
		}
		JSON(w, http.StatusOK, dryRunResponse{
			Rows:         rows,
			TotalRows:    parsed.TotalRows,
			ValidRows:    parsed.ValidRows,
			InvalidCount: parsed.InvalidRows,
			WouldInsert:  wouldInsert,
			WouldUpdate:  wouldUpdate,
		})
		return
	}

	importResult, err := h.svc.ImportCSV(r.Context(), info.UserID, header.Filename, parsed, defaultDeckID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "import failed")
		return
	}
	JSON(w, http.StatusOK, importResult)
}

// --- Helpers ---

// validateOptionalFields sanitises and length-checks the optional topic and
// source pointer fields. Returns (nil, nil, nil) when both are absent.
func validateOptionalFields(topic, source *string) (*string, *string, error) {
	var outTopic, outSource *string

	if topic != nil {
		v, err := validate.StringField("topic", *topic, model.MaxTopicLen)
		if err != nil {
			return nil, nil, err
		}
		if v != "" {
			outTopic = &v
		}
	}
	if source != nil {
		v, err := validate.StringField("source", *source, model.MaxSourceLen)
		if err != nil {
			return nil, nil, err
		}
		if v != "" {
			outSource = &v
		}
	}
	return outTopic, outSource, nil
}
