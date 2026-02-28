package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"

	"webapp/internal/model"
)

// ErrCardQuestionTaken is returned when a card's question violates the
// UNIQUE(deck_id, question) constraint.
var ErrCardQuestionTaken = errors.New("question already exists in this deck")

type CardRepo struct{ db DBTX }

func NewCardRepo(db DBTX) *CardRepo { return &CardRepo{db: db} }

func (r *CardRepo) Create(ctx context.Context, c model.Card) (model.Card, error) {
	const q = `
		INSERT INTO cards (deck_id, topic, type, question, answer, source)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, deck_id, topic, type, question, answer, source, created_at, updated_at`

	var out model.Card
	var topic, source sql.NullString
	err := r.db.QueryRowContext(ctx, q,
		c.DeckID, toNullString(c.Topic), string(c.Type), c.Question, c.Answer, toNullString(c.Source),
	).Scan(
		&out.ID, &out.DeckID, &topic, &out.Type, &out.Question, &out.Answer, &source,
		&out.CreatedAt, &out.UpdatedAt,
	)
	out.Topic = toStringPtr(topic)
	out.Source = toStringPtr(source)
	if err != nil {
		if isUniqueViolation(err) {
			return model.Card{}, ErrCardQuestionTaken
		}
		return model.Card{}, fmt.Errorf("card create: %w", err)
	}
	return out, nil
}

// Upsert inserts a card or, on a (deck_id, question) conflict, updates the
// mutable fields (type, answer, topic, source, updated_at).
// The boolean wasInserted is true when the row was newly inserted and false
// when an existing row was updated.
//
// The xmax trick is used: PostgreSQL sets xmax = 0 for newly inserted rows and
// xmax = current transaction ID (non-zero) for updated rows.
func (r *CardRepo) Upsert(ctx context.Context, c model.Card) (model.Card, bool, error) {
	const q = `
		INSERT INTO cards (deck_id, topic, type, question, answer, source)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (deck_id, question) DO UPDATE SET
			type       = EXCLUDED.type,
			answer     = EXCLUDED.answer,
			topic      = EXCLUDED.topic,
			source     = EXCLUDED.source,
			updated_at = now()
		RETURNING id, deck_id, topic, type, question, answer, source,
		          created_at, updated_at,
		          (xmax = 0) AS was_inserted`

	var out model.Card
	var topic, source sql.NullString
	var wasInserted bool
	err := r.db.QueryRowContext(ctx, q,
		c.DeckID, toNullString(c.Topic), string(c.Type), c.Question, c.Answer, toNullString(c.Source),
	).Scan(
		&out.ID, &out.DeckID, &topic, &out.Type, &out.Question, &out.Answer, &source,
		&out.CreatedAt, &out.UpdatedAt, &wasInserted,
	)
	out.Topic = toStringPtr(topic)
	out.Source = toStringPtr(source)
	if err != nil {
		return model.Card{}, false, fmt.Errorf("card upsert: %w", err)
	}
	return out, wasInserted, nil
}

// ExistingQuestions returns the subset of questions that already exist in the
// given deck. Used by dry-run estimation to predict insert vs update counts.
func (r *CardRepo) ExistingQuestions(ctx context.Context, deckID string, questions []string) (map[string]bool, error) {
	if len(questions) == 0 {
		return make(map[string]bool), nil
	}
	const q = `SELECT question FROM cards WHERE deck_id = $1 AND question = ANY($2)`

	rows, err := r.db.QueryContext(ctx, q, deckID, pq.Array(questions))
	if err != nil {
		return nil, fmt.Errorf("card existing questions: %w", err)
	}
	defer rows.Close()

	existing := make(map[string]bool, len(questions))
	for rows.Next() {
		var question string
		if err := rows.Scan(&question); err != nil {
			return nil, fmt.Errorf("card existing questions scan: %w", err)
		}
		existing[question] = true
	}
	return existing, rows.Err()
}

func (r *CardRepo) FindByID(ctx context.Context, id string) (model.Card, error) {
	const q = `SELECT id, deck_id, topic, type, question, answer, source, created_at, updated_at
	           FROM cards WHERE id = $1`

	var c model.Card
	var topic, source sql.NullString
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&c.ID, &c.DeckID, &topic, &c.Type, &c.Question, &c.Answer, &source,
		&c.CreatedAt, &c.UpdatedAt,
	)
	c.Topic = toStringPtr(topic)
	c.Source = toStringPtr(source)
	if err != nil {
		return model.Card{}, fmt.Errorf("card find by id: %w", err)
	}
	return c, nil
}

// CardListParams parameterises ListByDeckPaged.
type CardListParams struct {
	DeckID      string
	SearchQuery string    // empty = no filter
	CursorTS    time.Time // zero = no cursor (first page)
	CursorID    string
	Limit       int
}

// ListByDeckPaged returns a page of CardListItem rows (answer excluded) for the
// given deck, ordered by (updated_at DESC, id DESC).
//
// It fetches Limit+1 rows so callers can detect whether a next page exists.
func (r *CardRepo) ListByDeckPaged(ctx context.Context, p CardListParams) ([]model.CardListItem, error) {
	var sb strings.Builder
	var args []any

	nextArg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	sb.WriteString(`SELECT id, deck_id, topic, type, question, updated_at FROM cards WHERE deck_id = `)
	sb.WriteString(nextArg(p.DeckID))

	if p.SearchQuery != "" {
		ph := nextArg(p.SearchQuery)
		fmt.Fprintf(&sb,
			` AND (question ILIKE '%%' || %s || '%%' OR topic ILIKE '%%' || %s || '%%')`,
			ph, ph)
	}

	if !p.CursorTS.IsZero() {
		tsArg := nextArg(p.CursorTS)
		idArg := nextArg(p.CursorID)
		fmt.Fprintf(&sb, ` AND (updated_at, id) < (%s::timestamptz, %s::uuid)`, tsArg, idArg)
	}

	sb.WriteString(` ORDER BY updated_at DESC, id DESC LIMIT `)
	sb.WriteString(nextArg(p.Limit))

	rows, err := r.db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("card list paged: %w", err)
	}
	defer rows.Close()

	var items []model.CardListItem
	for rows.Next() {
		var c model.CardListItem
		var topic sql.NullString
		if err := rows.Scan(&c.ID, &c.DeckID, &topic, &c.Type, &c.Question, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("card list scan: %w", err)
		}
		c.Topic = toStringPtr(topic)
		items = append(items, c)
	}
	return items, rows.Err()
}

func (r *CardRepo) ListByDeck(ctx context.Context, deckID string) ([]model.Card, error) {
	const q = `SELECT id, deck_id, topic, type, question, answer, source, created_at, updated_at
	           FROM cards WHERE deck_id = $1 ORDER BY created_at`

	rows, err := r.db.QueryContext(ctx, q, deckID)
	if err != nil {
		return nil, fmt.Errorf("card list: %w", err)
	}
	defer rows.Close()

	var cards []model.Card
	for rows.Next() {
		var c model.Card
		var topic, source sql.NullString
		if err := rows.Scan(
			&c.ID, &c.DeckID, &topic, &c.Type, &c.Question, &c.Answer, &source,
			&c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("card scan: %w", err)
		}
		c.Topic = toStringPtr(topic)
		c.Source = toStringPtr(source)
		cards = append(cards, c)
	}
	return cards, rows.Err()
}

func (r *CardRepo) Update(ctx context.Context, c model.Card) error {
	const q = `
		UPDATE cards
		SET topic = $2, type = $3, question = $4, answer = $5, source = $6, updated_at = now()
		WHERE id = $1`

	res, err := r.db.ExecContext(ctx, q,
		c.ID, toNullString(c.Topic), string(c.Type), c.Question, c.Answer, toNullString(c.Source),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrCardQuestionTaken
		}
		return fmt.Errorf("card update: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *CardRepo) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM cards WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("card delete: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// FindByQuestion looks up a card by deck + exact question text.
func (r *CardRepo) FindByQuestion(ctx context.Context, deckID, question string) (model.Card, error) {
	const q = `SELECT id, deck_id, topic, type, question, answer, source, created_at, updated_at
	           FROM cards WHERE deck_id = $1 AND question = $2`

	var c model.Card
	var topic, source sql.NullString
	err := r.db.QueryRowContext(ctx, q, deckID, question).Scan(
		&c.ID, &c.DeckID, &topic, &c.Type, &c.Question, &c.Answer, &source,
		&c.CreatedAt, &c.UpdatedAt,
	)
	c.Topic = toStringPtr(topic)
	c.Source = toStringPtr(source)
	if err != nil {
		return model.Card{}, fmt.Errorf("card find by question: %w", err)
	}
	return c, nil
}
