package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"webapp/internal/csvparse"
	"webapp/internal/middleware"
	"webapp/internal/model"
	"webapp/internal/pagination"
	"webapp/internal/repository"
)

// ErrForbidden is returned when a professor tries to mutate a deck they don't own.
var ErrForbidden = errors.New("forbidden: you do not own this deck")

type ContentService struct {
	db      *sql.DB
	decks   *repository.DeckRepo
	cards   *repository.CardRepo
	uploads *repository.UploadRepo
}

func NewContentService(
	db *sql.DB,
	decks *repository.DeckRepo,
	cards *repository.CardRepo,
	uploads *repository.UploadRepo,
) *ContentService {
	return &ContentService{db: db, decks: decks, cards: cards, uploads: uploads}
}

// --- Deck CRUD ---

// authFromCtx extracts AuthInfo from the context. Returns zero-value if absent.
func authFromCtx(ctx context.Context) model.AuthInfo {
	info, _ := middleware.GetAuthInfo(ctx)
	return info
}

// checkDeckOwnership fetches the deck and enforces ownership for non-admins.
// Admins may mutate any deck. Professors may only mutate decks they created.
// Returns the deck on success so callers avoid a second DB round-trip.
func (s *ContentService) checkDeckOwnership(ctx context.Context, deckID string) (model.Deck, error) {
	deck, err := s.decks.FindByID(ctx, deckID)
	if err != nil {
		return model.Deck{}, err
	}
	auth := authFromCtx(ctx)
	if auth.HasAnyRole("admin") {
		return deck, nil // admins bypass ownership check
	}
	if !deck.IsOwnedBy(auth.UserID) {
		return model.Deck{}, ErrForbidden
	}
	return deck, nil
}

// checkCardDeckOwnership fetches the card's parent deck and enforces ownership.
func (s *ContentService) checkCardDeckOwnership(ctx context.Context, deckID string) error {
	_, err := s.checkDeckOwnership(ctx, deckID)
	return err
}

func (s *ContentService) CreateDeck(ctx context.Context, name string, desc, subject *string) (model.Deck, error) {
	auth := authFromCtx(ctx)
	return s.decks.Create(ctx, name, desc, subject, auth.UserID, false)
}

func (s *ContentService) UpdateDeck(ctx context.Context, id, name string, desc, subject *string) (model.Deck, error) {
	if _, err := s.checkDeckOwnership(ctx, id); err != nil {
		return model.Deck{}, err
	}
	return s.decks.Update(ctx, id, name, desc, subject)
}

// GetDeck returns a deck, enforcing ownership for non-admins.
func (s *ContentService) GetDeck(ctx context.Context, id string) (model.Deck, error) {
	return s.checkDeckOwnership(ctx, id)
}

// PatchDeck updates is_active and/or expires_at on a deck.
// req.ExpiresAt == "" clears the expiry; a valid RFC3339 string sets it; nil leaves it unchanged.
func (s *ContentService) PatchDeck(ctx context.Context, id string, req model.PatchDeckRequest) (model.Deck, error) {
	if _, err := s.checkDeckOwnership(ctx, id); err != nil {
		return model.Deck{}, err
	}

	var expiresAt *time.Time
	clearExpiry := false

	if req.ExpiresAt != nil {
		if *req.ExpiresAt == "" {
			clearExpiry = true
		} else {
			t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
			if err != nil {
				return model.Deck{}, fmt.Errorf("invalid expires_at format, use RFC3339: %w", err)
			}
			expiresAt = &t
		}
	}

	return s.decks.Patch(ctx, id, req.IsActive, expiresAt, clearExpiry)
}

func (s *ContentService) DeleteDeck(ctx context.Context, id string) error {
	if _, err := s.checkDeckOwnership(ctx, id); err != nil {
		return err
	}
	// Cards and reviews are removed automatically by ON DELETE CASCADE in the DB.
	return s.decks.Delete(ctx, id)
}

// --- Card CRUD ---

func (s *ContentService) CreateCard(ctx context.Context, c model.Card) (model.Card, error) {
	if err := s.checkCardDeckOwnership(ctx, c.DeckID); err != nil {
		return model.Card{}, err
	}
	return s.cards.Create(ctx, c)
}

func (s *ContentService) UpdateCard(ctx context.Context, c model.Card) error {
	// Fetch current card to resolve its deck, then check ownership.
	existing, err := s.cards.FindByID(ctx, c.ID)
	if err != nil {
		return err
	}
	if err := s.checkCardDeckOwnership(ctx, existing.DeckID); err != nil {
		return err
	}
	return s.cards.Update(ctx, c)
}

// ListCards returns a paginated page of card list items (no answer) for a deck.
// Pass a zero cursorTS to start from the first page.
func (s *ContentService) ListCards(
	ctx context.Context,
	deckID, searchQuery string,
	cursorTS time.Time, cursorID string,
	limit int,
) (pagination.Page[model.CardListItem], error) {
	if _, err := s.checkDeckOwnership(ctx, deckID); err != nil {
		return pagination.Page[model.CardListItem]{}, err
	}
	items, err := s.cards.ListByDeckPaged(ctx, repository.CardListParams{
		DeckID:      deckID,
		SearchQuery: searchQuery,
		CursorTS:    cursorTS,
		CursorID:    cursorID,
		Limit:       limit + 1, // fetch one extra to detect next page
	})
	if err != nil {
		return pagination.Page[model.CardListItem]{}, err
	}

	var nextCursor *string
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		c := pagination.EncodeTimestampIDCursor(last.UpdatedAt, last.ID)
		nextCursor = &c
	}

	// Ensure items is never nil so JSON encodes as [] instead of null.
	if items == nil {
		items = []model.CardListItem{}
	}

	return pagination.Page[model.CardListItem]{
		Items:      items,
		NextCursor: nextCursor,
	}, nil
}

// GetCard returns the full card (including answer), enforcing deck ownership for non-admins.
func (s *ContentService) GetCard(ctx context.Context, id string) (model.Card, error) {
	card, err := s.cards.FindByID(ctx, id)
	if err != nil {
		return model.Card{}, err
	}
	if _, err := s.checkDeckOwnership(ctx, card.DeckID); err != nil {
		return model.Card{}, err
	}
	return card, nil
}

func (s *ContentService) DeleteCard(ctx context.Context, id string) error {
	existing, err := s.cards.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.checkCardDeckOwnership(ctx, existing.DeckID); err != nil {
		return err
	}
	return s.cards.Delete(ctx, id)
}

// --- CSV Import ---

// EstimateDryRun counts how many valid rows from parsed would result in inserts
// vs updates if the import were executed now.
//
// It intentionally does NOT create decks; missing decks are treated as
// "all rows would insert". defaultDeckID is used when rows have an empty Deck
// field (single-deck mode).
func (s *ContentService) EstimateDryRun(ctx context.Context, parsed *csvparse.Result, defaultDeckID string) (wouldInsert, wouldUpdate int, err error) {
	// Group valid questions by the deck they belong to.
	byDeck := make(map[string][]string) // deckName/ID key → questions
	for _, row := range parsed.Rows {
		if row.Status != "ok" {
			continue
		}
		key := row.Deck // may be "" in single-deck mode
		byDeck[key] = append(byDeck[key], row.Question)
	}

	for deckKey, questions := range byDeck {
		var deckID string

		if deckKey == "" {
			// Single-deck mode: the deck is already known by ID.
			deckID = defaultDeckID
		} else {
			deck, findErr := s.decks.FindByName(ctx, deckKey)
			if findErr != nil {
				// Deck not in DB yet → all rows would be inserts.
				wouldInsert += len(questions)
				continue
			}
			deckID = deck.ID
		}

		existing, findErr := s.cards.ExistingQuestions(ctx, deckID, questions)
		if findErr != nil {
			return 0, 0, findErr
		}
		for _, q := range questions {
			if existing[q] {
				wouldUpdate++
			} else {
				wouldInsert++
			}
		}
	}
	return wouldInsert, wouldUpdate, nil
}

// ImportCSV executes a transactional import of all valid rows from parsed.
//
//   - Each deck is upserted by name inside the transaction.
//   - Each card is upserted by (deck_id, question); on conflict the mutable
//     fields (type, answer, topic, source) are updated.
//   - Invalid rows are counted but do not abort the transaction.
//   - defaultDeckID is used when rows have an empty Deck field (single-deck mode).
func (s *ContentService) ImportCSV(ctx context.Context, userID, filename string, parsed *csvparse.Result, defaultDeckID string) (result model.ImportResult, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.ImportResult{}, fmt.Errorf("begin import transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	deckTx := repository.NewDeckRepo(tx)
	cardTx := repository.NewCardRepo(tx)

	var imported, updated, invalid, decksCreated int
	deckCache := make(map[string]model.Deck) // deckName → Deck

	// If single-deck mode, pre-populate cache with the known deck so we don't
	// attempt an upsert for every row.
	if defaultDeckID != "" {
		if dk, fetchErr := s.decks.FindByID(ctx, defaultDeckID); fetchErr == nil {
			deckCache[""] = dk
		}
	}

	for _, row := range parsed.Rows {
		if row.Status != "ok" {
			invalid++
			continue
		}

		deck, ok := deckCache[row.Deck]
		if !ok {
			var created bool
			var innerErr error
			deck, created, innerErr = deckTx.FindOrCreateByName(ctx, row.Deck, userID)
			if innerErr != nil {
				invalid++
				continue
			}
			// Apply subject from CSV when the deck was just created or has no subject yet.
			if row.Subject != "" && (created || deck.Subject == nil) {
				subj := row.Subject
				if updated, updErr := deckTx.Update(ctx, deck.ID, deck.Name, deck.Description, &subj); updErr == nil {
					deck = updated
				}
			}
			deckCache[row.Deck] = deck
			if created {
				decksCreated++
			}
		}

		card := model.Card{
			DeckID:   deck.ID,
			Type:     model.CardType(row.Type),
			Question: row.Question,
			Answer:   row.Answer,
		}
		if row.Topic != "" {
			card.Topic = &row.Topic
		}
		if row.Source != "" {
			card.Source = &row.Source
		}

		_, wasInserted, upsertErr := cardTx.Upsert(ctx, card)
		if upsertErr != nil {
			invalid++
			continue
		}
		if wasInserted {
			imported++
		} else {
			updated++
		}
	}

	if err = tx.Commit(); err != nil {
		return model.ImportResult{}, fmt.Errorf("commit import transaction: %w", err)
	}

	// Determine a single deck_id for the upload record when the import touched
	// exactly one deck; otherwise leave it nil (multi-deck import).
	var uploadDeckID *string
	if len(deckCache) == 1 {
		for _, dk := range deckCache {
			id := dk.ID
			uploadDeckID = &id
		}
	}

	_, _ = s.uploads.Create(ctx, model.Upload{
		UserID:        userID,
		DeckID:        uploadDeckID,
		Filename:      filename,
		ImportedCount: imported,
		UpdatedCount:  updated,
		InvalidCount:  invalid,
		DecksCreated:  decksCreated,
	})

	return model.ImportResult{
		ImportedCount: imported,
		UpdatedCount:  updated,
		InvalidCount:  invalid,
		DecksCreated:  decksCreated,
	}, nil
}

// --- Student private deck management ---

// ListMyDecks returns all private decks for the calling user (including empty ones).
func (s *ContentService) ListMyDecks(ctx context.Context) ([]model.DeckWithCount, error) {
	auth := authFromCtx(ctx)
	return s.decks.ListPrivateByOwner(ctx, auth.UserID)
}

// CreateMyDeck creates a private deck owned by the calling student.
func (s *ContentService) CreateMyDeck(ctx context.Context, name string, desc *string) (model.Deck, error) {
	auth := authFromCtx(ctx)
	return s.decks.Create(ctx, name, desc, nil, auth.UserID, true)
}

// DeleteMyDeck deletes a private deck, enforcing that the caller is the owner.
func (s *ContentService) DeleteMyDeck(ctx context.Context, deckID string) error {
	deck, err := s.decks.FindByID(ctx, deckID)
	if err != nil {
		return err
	}
	auth := authFromCtx(ctx)
	if !deck.IsOwnedBy(auth.UserID) || !deck.IsPrivate {
		return ErrForbidden
	}
	return s.decks.Delete(ctx, deckID)
}

// checkMyDeckOwnership verifies the deck is private and owned by the calling user.
func (s *ContentService) checkMyDeckOwnership(ctx context.Context, deckID string) (model.Deck, error) {
	deck, err := s.decks.FindByID(ctx, deckID)
	if err != nil {
		return model.Deck{}, err
	}
	auth := authFromCtx(ctx)
	if !deck.IsOwnedBy(auth.UserID) || !deck.IsPrivate {
		return model.Deck{}, ErrForbidden
	}
	return deck, nil
}

// checkMyCardOwnership fetches a card and verifies ownership of its parent private deck.
func (s *ContentService) checkMyCardOwnership(ctx context.Context, cardID string) (model.Card, error) {
	card, err := s.cards.FindByID(ctx, cardID)
	if err != nil {
		return model.Card{}, err
	}
	if _, err := s.checkMyDeckOwnership(ctx, card.DeckID); err != nil {
		return model.Card{}, err
	}
	return card, nil
}

// ListMyDeckCards returns all cards for a student's private deck.
func (s *ContentService) ListMyDeckCards(ctx context.Context, deckID string) ([]model.Card, error) {
	if _, err := s.checkMyDeckOwnership(ctx, deckID); err != nil {
		return nil, err
	}
	return s.cards.ListByDeck(ctx, deckID)
}

// CreateMyCard adds a card to a student's private deck.
func (s *ContentService) CreateMyCard(ctx context.Context, deckID, question, answer string, cardType model.CardType) (model.Card, error) {
	if _, err := s.checkMyDeckOwnership(ctx, deckID); err != nil {
		return model.Card{}, err
	}
	return s.cards.Create(ctx, model.Card{
		DeckID:   deckID,
		Type:     cardType,
		Question: question,
		Answer:   answer,
	})
}

// UpdateMyCard updates a card belonging to a student's private deck.
func (s *ContentService) UpdateMyCard(ctx context.Context, cardID, question, answer string, cardType model.CardType) error {
	card, err := s.checkMyCardOwnership(ctx, cardID)
	if err != nil {
		return err
	}
	card.Question = question
	card.Answer = answer
	card.Type = cardType
	return s.cards.Update(ctx, card)
}

// DeleteMyCard removes a card from a student's private deck.
func (s *ContentService) DeleteMyCard(ctx context.Context, cardID string) error {
	if _, err := s.checkMyCardOwnership(ctx, cardID); err != nil {
		return err
	}
	return s.cards.Delete(ctx, cardID)
}

// ExportDeckCSV returns all cards for a deck ordered by created_at.
// The deck name is resolved so callers can use it in the Content-Disposition header.
// Non-admins may only export decks they own.
func (s *ContentService) ExportDeckCSV(ctx context.Context, deckID string) (deckName string, cards []model.Card, err error) {
	deck, err := s.checkDeckOwnership(ctx, deckID)
	if err != nil {
		return "", nil, err
	}
	cards, err = s.cards.ListByDeck(ctx, deckID)
	if err != nil {
		return "", nil, fmt.Errorf("export list: %w", err)
	}
	return deck.Name, cards, nil
}
