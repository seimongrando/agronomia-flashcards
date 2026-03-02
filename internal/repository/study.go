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
	// ShowAll skips the is_active / expires_at filter (professor/admin management views).
	ShowAll bool
	// HideEmpty filters out decks with no cards (home page). Management views set false.
	HideEmpty bool
	// ApplyVisibility enforces student-facing visibility rules:
	//   - user's own private decks
	//   - decks in classes the user is enrolled in
	//   - general decks (not private, no class assignments)
	// When false (management view) all matching decks are returned without class filter.
	ApplyVisibility bool
}

// ListDecksWithCountsPaged returns a page of decks ordered by (name ASC, id ASC),
// enriched with per-user card counts and optional class context.
func (r *StudyRepo) ListDecksWithCountsPaged(ctx context.Context, p DeckListParams) ([]model.DeckWithCounts, error) {
	var sb strings.Builder
	var args []any

	nextArg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	uid := nextArg(p.UserID)

	// Class context subqueries — only emitted when ApplyVisibility is set
	// so that the student sees which class each deck belongs to.
	classIDExpr := "NULL::text"
	classNameExpr := "NULL::text"
	if p.ApplyVisibility {
		classIDExpr = fmt.Sprintf(`(
			SELECT cl.id FROM class_decks cd
			JOIN classes cl ON cl.id = cd.class_id
			JOIN class_members cm ON cm.class_id = cd.class_id AND cm.user_id = %s
			WHERE cd.deck_id = d.id ORDER BY cl.name ASC LIMIT 1
		)`, uid)
		classNameExpr = fmt.Sprintf(`(
			SELECT cl.name FROM class_decks cd
			JOIN classes cl ON cl.id = cd.class_id
			JOIN class_members cm ON cm.class_id = cd.class_id AND cm.user_id = %s
			WHERE cd.deck_id = d.id ORDER BY cl.name ASC LIMIT 1
		)`, uid)
	}

	fmt.Fprintf(&sb, `
		SELECT d.id, d.name, d.description, d.subject, d.is_active, d.is_private,
		       d.expires_at, d.created_at, d.created_by,
		       %s AS class_id, %s AS class_name,
		       COUNT(c.id)::int AS total_cards,
		       COUNT(CASE WHEN c.id IS NOT NULL AND (rv.id IS NULL OR rv.next_due <= now()) THEN 1 END)::int AS due_now,
		       MAX(rv.updated_at) AS last_studied,
		       MIN(rv.next_due) FILTER (WHERE rv.next_due > now()) AS next_review
		FROM decks d
		LEFT JOIN cards c    ON c.deck_id = d.id
		LEFT JOIN reviews rv ON rv.card_id = c.id AND rv.user_id = %s
		WHERE 1=1`, classIDExpr, classNameExpr, uid)

	if !p.ShowAll {
		sb.WriteString(` AND d.is_active = true AND (d.expires_at IS NULL OR d.expires_at > now())`)
	}

	// Student visibility: private-owned OR in-class OR general (no class assignment).
	if p.ApplyVisibility {
		fmt.Fprintf(&sb, `
		AND (
		  (d.is_private = true AND d.created_by = %s)
		  OR EXISTS (
		    SELECT 1 FROM class_decks cd
		    JOIN class_members cm ON cm.class_id = cd.class_id
		    WHERE cd.deck_id = d.id AND cm.user_id = %s
		  )
		  OR (
		    d.is_private = false
		    AND NOT EXISTS (SELECT 1 FROM class_decks cd WHERE cd.deck_id = d.id)
		  )
		)`, uid, uid)
	}

	if p.CursorName != "" || p.CursorID != "" {
		nameArg := nextArg(p.CursorName)
		idArg := nextArg(p.CursorID)
		fmt.Fprintf(&sb,
			` AND (d.name > %s OR (d.name = %s AND d.id::text > %s))`,
			nameArg, nameArg, idArg)
	}

	sb.WriteString(` GROUP BY d.id`)

	if p.HideEmpty {
		sb.WriteString(` HAVING COUNT(c.id) > 0`)
	}

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
		var desc, subj, createdBy sql.NullString
		var classID, className sql.NullString
		var exp sql.NullTime
		var lastStudied, nextRv sql.NullTime
		if err := rows.Scan(
			&d.ID, &d.Name, &desc, &subj, &d.IsActive, &d.IsPrivate,
			&exp, &d.CreatedAt, &createdBy,
			&classID, &className,
			&d.TotalCards, &d.DueNow, &lastStudied, &nextRv,
		); err != nil {
			return nil, fmt.Errorf("deck counts scan: %w", err)
		}
		d.Description = toStringPtr(desc)
		d.Subject = toStringPtr(subj)
		d.CreatedBy = toStringPtr(createdBy)
		d.ClassID = toStringPtr(classID)
		d.ClassName = toStringPtr(className)
		if exp.Valid {
			t := exp.Time
			d.ExpiresAt = &t
		}
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

	// ── Study streak ─────────────────────────────────────────────────────────
	// Computes current (consecutive ending today/yesterday) and longest streaks
	// using a gaps-and-islands approach on distinct study dates.
	const streakQ = `
		WITH daily AS (
			SELECT DISTINCT (updated_at AT TIME ZONE 'UTC')::date AS d
			FROM reviews
			WHERE user_id = $1
		),
		gaps AS (
			SELECT d, d - (ROW_NUMBER() OVER (ORDER BY d))::int AS grp
			FROM daily
		),
		groups AS (
			SELECT grp,
			       MAX(d)     AS last_day,
			       COUNT(*)::int AS len
			FROM gaps
			GROUP BY grp
		)
		SELECT
			COALESCE(
				(SELECT len FROM groups
				 WHERE last_day >= CURRENT_DATE - 1
				 ORDER BY last_day DESC LIMIT 1),
				0
			) AS current_streak,
			COALESCE(MAX(len), 0) AS longest_streak
		FROM groups`

	if err = r.db.QueryRowContext(ctx, streakQ, userID).Scan(
		&s.StudyStreak, &s.LongestStreak,
	); err != nil {
		return model.ProgressStats{}, fmt.Errorf("streak: %w", err)
	}

	// ── Per-deck breakdown ───────────────────────────────────────────────────
	const deckQ = `
		SELECT
			d.id,
			d.name,
			COUNT(c.id)::int                                                       AS total_cards,
			COUNT(rv.id) FILTER (WHERE rv.streak >= 3)::int                        AS mastered,
			COUNT(rv.id) FILTER (WHERE rv.streak > 0 AND rv.streak < 3)::int       AS learning,
			COUNT(c.id)  FILTER (WHERE rv.id IS NULL OR rv.next_due <= now())::int  AS due_now,
			COUNT(rv.id) FILTER (WHERE rv.last_result = 0)::int                    AS wrong,
			COUNT(rv.id) FILTER (WHERE rv.last_result = 1)::int                    AS hard
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
		if err := rows.Scan(&dp.ID, &dp.Name, &dp.TotalCards, &dp.Mastered, &dp.Learning, &dp.DueNow, &dp.Wrong, &dp.Hard); err != nil {
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

// ProfessorStats returns aggregate content and engagement metrics for the
// professor/admin dashboard. No individual student data is exposed.
func (r *StudyRepo) ProfessorStats(ctx context.Context) (model.ProfessorStats, error) {
	var s model.ProfessorStats

	// ── Overall counts ──────────────────────────────────────────────────────
	err := r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*)                                                            AS total_decks,
			COUNT(*) FILTER (WHERE is_active AND (expires_at IS NULL OR expires_at > now())) AS active_decks
		FROM decks`).Scan(&s.TotalDecks, &s.ActiveDecks)
	if err != nil {
		return s, fmt.Errorf("professor stats totals: %w", err)
	}

	if err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cards`).Scan(&s.TotalCards); err != nil {
		return s, fmt.Errorf("professor stats cards: %w", err)
	}

	if err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT user_id) FROM reviews WHERE updated_at >= now() - INTERVAL '30 days'`,
	).Scan(&s.ActiveStudents); err != nil {
		return s, fmt.Errorf("professor stats active students: %w", err)
	}

	if err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM reviews`).Scan(&s.TotalReviews); err != nil {
		return s, fmt.Errorf("professor stats reviews: %w", err)
	}

	// ── Per-deck breakdown ──────────────────────────────────────────────────
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			d.id,
			d.name,
			d.subject,
			d.is_active,
			COUNT(DISTINCT c.id)::int                                                         AS total_cards,
			COUNT(DISTINCT rv.user_id)::int                                                   AS students_studying,
			COALESCE(ROUND(AVG(
				CASE rv.last_result WHEN 2 THEN 100.0 WHEN 1 THEN 50.0 ELSE 0.0 END
			))::int, 0)                                                                       AS avg_accuracy,
			COUNT(rv.id)::int                                                                 AS total_reviews
		FROM decks d
		LEFT JOIN cards   c  ON c.deck_id = d.id
		LEFT JOIN reviews rv ON rv.card_id = c.id
		GROUP BY d.id
		ORDER BY students_studying DESC, d.name`)
	if err != nil {
		return s, fmt.Errorf("professor stats decks: %w", err)
	}
	defer rows.Close()

	s.Decks = []model.DeckStat{}
	for rows.Next() {
		var ds model.DeckStat
		var subj sql.NullString
		if err := rows.Scan(&ds.ID, &ds.Name, &subj, &ds.IsActive,
			&ds.TotalCards, &ds.StudentsStudying, &ds.AvgAccuracy, &ds.TotalReviews); err != nil {
			return s, fmt.Errorf("professor stats deck scan: %w", err)
		}
		if subj.Valid {
			ds.Subject = &subj.String
		}
		s.Decks = append(s.Decks, ds)
	}
	if err := rows.Err(); err != nil {
		return s, err
	}

	// ── Hardest cards (≥5 reviews, lowest accuracy) ─────────────────────────
	hrows, err := r.db.QueryContext(ctx, `
		SELECT
			c.id,
			c.question,
			c.type,
			d.name AS deck_name,
			COUNT(rv.id)::int AS total_reviews,
			ROUND(AVG(
				CASE rv.last_result WHEN 2 THEN 100.0 WHEN 1 THEN 50.0 ELSE 0.0 END
			))::int AS accuracy
		FROM cards   c
		JOIN decks   d  ON d.id = c.deck_id
		JOIN reviews rv ON rv.card_id = c.id
		GROUP BY c.id, c.question, c.type, d.name
		HAVING COUNT(rv.id) >= 5
		ORDER BY accuracy ASC, total_reviews DESC
		LIMIT 10`)
	if err != nil {
		return s, fmt.Errorf("professor stats hard cards: %w", err)
	}
	defer hrows.Close()

	s.HardestCards = []model.HardCard{}
	for hrows.Next() {
		var hc model.HardCard
		if err := hrows.Scan(&hc.ID, &hc.Question, &hc.Type, &hc.DeckName,
			&hc.TotalReviews, &hc.Accuracy); err != nil {
			return s, fmt.Errorf("professor stats hard card scan: %w", err)
		}
		s.HardestCards = append(s.HardestCards, hc)
	}
	return s, hrows.Err()
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
