package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"webapp/internal/csvparse"
	"webapp/internal/model"
	"webapp/internal/pagination"
	"webapp/internal/repository"
)

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

func (s *ContentService) CreateDeck(ctx context.Context, name string, desc *string) (model.Deck, error) {
	return s.decks.Create(ctx, name, desc)
}

func (s *ContentService) UpdateDeck(ctx context.Context, id, name string, desc *string) (model.Deck, error) {
	return s.decks.Update(ctx, id, name, desc)
}

func (s *ContentService) GetDeck(ctx context.Context, id string) (model.Deck, error) {
	return s.decks.FindByID(ctx, id)
}

func (s *ContentService) DeleteDeck(ctx context.Context, id string) error {
	count, err := s.decks.CardCount(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("deck has %d card(s); remove them before deleting the deck", count)
	}
	return s.decks.Delete(ctx, id)
}

// --- Card CRUD ---

func (s *ContentService) CreateCard(ctx context.Context, c model.Card) (model.Card, error) {
	if _, err := s.decks.FindByID(ctx, c.DeckID); err != nil {
		return model.Card{}, fmt.Errorf("deck not found: %w", err)
	}
	return s.cards.Create(ctx, c)
}

func (s *ContentService) UpdateCard(ctx context.Context, c model.Card) error {
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

func (s *ContentService) GetCard(ctx context.Context, id string) (model.Card, error) {
	return s.cards.FindByID(ctx, id)
}

func (s *ContentService) DeleteCard(ctx context.Context, id string) error {
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
			deck, created, innerErr = deckTx.FindOrCreateByName(ctx, row.Deck)
			if innerErr != nil {
				invalid++
				continue
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
