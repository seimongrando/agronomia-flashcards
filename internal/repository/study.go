package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"webapp/internal/model"
)

type StudyRepo struct{ db DBTX }

func NewStudyRepo(db DBTX) *StudyRepo { return &StudyRepo{db: db} }

// DeckListParams parameterises ListDecksWithCountsPaged.
type DeckListParams struct {
	UserID     string
	CursorName string // empty = first page
	CursorID   string
	Limit      int
}

// ListDecksWithCountsPaged returns a page of decks ordered by (name ASC, id ASC),
// enriched with per-user card counts.
// It fetches Limit+1 rows so the caller can detect whether a next page exists.
func (r *StudyRepo) ListDecksWithCountsPaged(ctx context.Context, p DeckListParams) ([]model.DeckWithCounts, error) {
	var sb strings.Builder
	var args []any

	nextArg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	uid := nextArg(p.UserID)
	sb.WriteString(`
		SELECT d.id, d.name, d.description, d.created_at,
		       COUNT(c.id)::int AS total_cards,
		       COUNT(CASE WHEN c.id IS NOT NULL AND (rv.id IS NULL OR rv.next_due <= now()) THEN 1 END)::int AS due_now,
		       MAX(rv.updated_at) AS last_studied,
		       MIN(rv.next_due) FILTER (WHERE rv.next_due > now()) AS next_review
		FROM decks d
		LEFT JOIN cards c    ON c.deck_id = d.id
		LEFT JOIN reviews rv ON rv.card_id = c.id AND rv.user_id = ` + uid + `
		WHERE 1=1`)

	if p.CursorName != "" || p.CursorID != "" {
		nameArg := nextArg(p.CursorName)
		idArg := nextArg(p.CursorID)
		fmt.Fprintf(&sb,
			` AND (d.name > %s OR (d.name = %s AND d.id::text > %s))`,
			nameArg, nameArg, idArg)
	}

	sb.WriteString(` GROUP BY d.id, d.name, d.description, d.created_at`)
	sb.WriteString(` ORDER BY d.name ASC, d.id ASC LIMIT `)
	sb.WriteString(nextArg(p.Limit))

	rows, err := r.db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("list decks with counts: %w", err)
	}
	defer rows.Close()

	var out []model.DeckWithCounts
	for rows.Next() {
		var d model.DeckWithCounts
		var desc sql.NullString
		var lastStudied, nextRv sql.NullTime
		if err := rows.Scan(&d.ID, &d.Name, &desc, &d.CreatedAt, &d.TotalCards, &d.DueNow, &lastStudied, &nextRv); err != nil {
			return nil, fmt.Errorf("deck counts scan: %w", err)
		}
		d.Description = toStringPtr(desc)
		if lastStudied.Valid {
			t := lastStudied.Time
			d.LastStudied = &t
		}
		if nextRv.Valid {
			t := nextRv.Time
			d.NextReview = &t
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ListDecksWithCounts is kept for backward compatibility.
func (r *StudyRepo) ListDecksWithCounts(ctx context.Context, userID string) ([]model.DeckWithCounts, error) {
	return r.ListDecksWithCountsPaged(ctx, DeckListParams{UserID: userID, Limit: 200})
}

// NextDueCard returns the card with the oldest due date (or never-reviewed cards first).
// Pass topic="" to study all topics.
func (r *StudyRepo) NextDueCard(ctx context.Context, userID, deckID, topic string) (model.Card, error) {
	var sb strings.Builder
	var args []any
	nextArg := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }

	sb.WriteString(`
		SELECT c.id, c.deck_id, c.topic, c.type, c.question, c.answer, c.source, c.created_at, c.updated_at
		FROM cards c
		LEFT JOIN reviews rv ON rv.card_id = c.id AND rv.user_id = ` + nextArg(userID) + `
		WHERE c.deck_id = ` + nextArg(deckID))
	if topic != "" {
		sb.WriteString(` AND c.topic = ` + nextArg(topic))
	}
	sb.WriteString(` AND (rv.id IS NULL OR rv.next_due <= now())
		ORDER BY COALESCE(rv.next_due, '1970-01-01'::timestamptz)
		LIMIT 1`)

	return scanCard(r.db.QueryRowContext(ctx, sb.String(), args...))
}

// NextRandomCard returns a random card from the deck.
// Pass topic="" to study all topics.
func (r *StudyRepo) NextRandomCard(ctx context.Context, deckID, topic string) (model.Card, error) {
	var sb strings.Builder
	var args []any
	nextArg := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }

	sb.WriteString(`SELECT id, deck_id, topic, type, question, answer, source, created_at, updated_at
		FROM cards WHERE deck_id = ` + nextArg(deckID))
	if topic != "" {
		sb.WriteString(` AND topic = ` + nextArg(topic))
	}
	sb.WriteString(` ORDER BY random() LIMIT 1`)

	return scanCard(r.db.QueryRowContext(ctx, sb.String(), args...))
}

// NextWrongCard returns the most recently wrong card (last 7 days).
// Pass topic="" to study all topics.
func (r *StudyRepo) NextWrongCard(ctx context.Context, userID, deckID, topic string) (model.Card, error) {
	var sb strings.Builder
	var args []any
	nextArg := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }

	sb.WriteString(`
		SELECT c.id, c.deck_id, c.topic, c.type, c.question, c.answer, c.source, c.created_at, c.updated_at
		FROM cards c
		INNER JOIN reviews rv ON rv.card_id = c.id AND rv.user_id = ` + nextArg(userID) + `
		WHERE c.deck_id = ` + nextArg(deckID))
	if topic != "" {
		sb.WriteString(` AND c.topic = ` + nextArg(topic))
	}
	sb.WriteString(`
		  AND rv.last_result = 0
		  AND rv.updated_at >= now() - interval '7 days'
		ORDER BY rv.updated_at DESC
		LIMIT 1`)

	return scanCard(r.db.QueryRowContext(ctx, sb.String(), args...))
}

func (r *StudyRepo) Stats(ctx context.Context, userID, deckID string) (model.StudyStats, error) {
	const q = `
		SELECT
			(SELECT COUNT(*)
			 FROM cards c
			 LEFT JOIN reviews rv ON rv.card_id = c.id AND rv.user_id = $1
			 WHERE c.deck_id = $2 AND (rv.id IS NULL OR rv.next_due <= now())
			)::int,
			(SELECT COUNT(*)
			 FROM reviews rv JOIN cards c ON c.id = rv.card_id
			 WHERE rv.user_id = $1 AND c.deck_id = $2 AND rv.updated_at >= CURRENT_DATE
			)::int,
			COALESCE(
				(SELECT ROUND(100.0 *
					COUNT(*) FILTER (WHERE rv.last_result = 2) /
					NULLIF(COUNT(*), 0))
				 FROM reviews rv JOIN cards c ON c.id = rv.card_id
				 WHERE rv.user_id = $1 AND c.deck_id = $2 AND rv.updated_at >= CURRENT_DATE
				), 0
			)::int,
			(SELECT COUNT(*) FROM cards WHERE deck_id = $2)::int`

	var s model.StudyStats
	err := r.db.QueryRowContext(ctx, q, userID, deckID).Scan(&s.DueNow, &s.ReviewedToday, &s.AccuracyPct, &s.TotalCards)
	if err != nil {
		return model.StudyStats{}, fmt.Errorf("stats: %w", err)
	}
	return s, nil
}

// GlobalProgress returns aggregate study statistics for a user.
func (r *StudyRepo) GlobalProgress(ctx context.Context, userID string) (model.ProgressStats, error) {
	// ── Global aggregates ────────────────────────────────────────────────────
	const globalQ = `
		SELECT
			COUNT(*)::int                                                          AS total_studied,
			COUNT(*) FILTER (WHERE streak >= 3)::int                              AS mastered,
			COUNT(*) FILTER (WHERE streak > 0 AND streak < 3)::int               AS learning,
			COUNT(*) FILTER (WHERE next_due <= now())::int                        AS due_today,
			COALESCE(
				ROUND(100.0 * COUNT(*) FILTER (WHERE last_result = 2 AND updated_at >= now() - interval '7 days')
					/ NULLIF(COUNT(*) FILTER (WHERE updated_at >= now() - interval '7 days'), 0)
				), 0
			)::int                                                                AS accuracy_7d,
			COUNT(DISTINCT updated_at::date)::int                                 AS study_days
		FROM reviews
		WHERE user_id = $1`

	var s model.ProgressStats
	err := r.db.QueryRowContext(ctx, globalQ, userID).Scan(
		&s.TotalStudied, &s.Mastered, &s.Learning, &s.DueToday, &s.Accuracy7d, &s.StudyDays,
	)
	if err != nil {
		return model.ProgressStats{}, fmt.Errorf("global progress: %w", err)
	}

	// ── Per-deck breakdown ───────────────────────────────────────────────────
	const deckQ = `
		SELECT
			d.id,
			d.name,
			COUNT(c.id)::int                                                      AS total_cards,
			COUNT(rv.id) FILTER (WHERE rv.streak >= 3)::int                       AS mastered,
			COUNT(rv.id) FILTER (WHERE rv.streak > 0 AND rv.streak < 3)::int      AS learning,
			COUNT(c.id)  FILTER (WHERE rv.id IS NULL OR rv.next_due <= now())::int AS due_now
		FROM decks d
		JOIN cards c    ON c.deck_id = d.id
		LEFT JOIN reviews rv ON rv.card_id = c.id AND rv.user_id = $1
		GROUP BY d.id, d.name
		HAVING COUNT(rv.id) > 0
		ORDER BY d.name`

	rows, err := r.db.QueryContext(ctx, deckQ, userID)
	if err != nil {
		return model.ProgressStats{}, fmt.Errorf("deck progress: %w", err)
	}
	defer rows.Close()

	s.Decks = []model.DeckProgress{}
	for rows.Next() {
		var dp model.DeckProgress
		if err := rows.Scan(&dp.ID, &dp.Name, &dp.TotalCards, &dp.Mastered, &dp.Learning, &dp.DueNow); err != nil {
			return model.ProgressStats{}, fmt.Errorf("deck progress scan: %w", err)
		}
		s.Decks = append(s.Decks, dp)
	}
	if err := rows.Err(); err != nil {
		return model.ProgressStats{}, fmt.Errorf("deck progress rows: %w", err)
	}
	return s, nil
}

// DeckTopics returns distinct non-null topic values for a deck, alphabetically sorted.
func (r *StudyRepo) DeckTopics(ctx context.Context, deckID string) ([]string, error) {
	const q = `
		SELECT DISTINCT topic FROM cards
		WHERE deck_id = $1 AND topic IS NOT NULL AND topic <> ''
		ORDER BY topic`

	rows, err := r.db.QueryContext(ctx, q, deckID)
	if err != nil {
		return nil, fmt.Errorf("deck topics: %w", err)
	}
	defer rows.Close()

	var topics []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, fmt.Errorf("topic scan: %w", err)
		}
		topics = append(topics, t)
	}
	if topics == nil {
		topics = []string{}
	}
	return topics, rows.Err()
}

// scanCard is a shared helper for scanning a single card row.
func scanCard(row *sql.Row) (model.Card, error) {
	var c model.Card
	var topic, source sql.NullString
	err := row.Scan(
		&c.ID, &c.DeckID, &topic, &c.Type, &c.Question, &c.Answer, &source,
		&c.CreatedAt, &c.UpdatedAt,
	)
	c.Topic = toStringPtr(topic)
	c.Source = toStringPtr(source)
	if err != nil {
		return model.Card{}, err
	}
	return c, nil
}
