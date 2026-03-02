package repository

import (
	"context"
	"database/sql"

	"webapp/internal/model"
)

// ClassStatsRepo handles aggregated reporting queries for classes.
type ClassStatsRepo struct{ db DBTX }

func NewClassStatsRepo(db DBTX) *ClassStatsRepo { return &ClassStatsRepo{db: db} }

// GetClassStats returns the full performance report for a class.
// All values are aggregated — no individual student data is exposed.
func (r *ClassStatsRepo) GetClassStats(ctx context.Context, classID string) (model.ClassStats, error) {
	var s model.ClassStats
	s.ClassID = classID

	// 1 ── Class name + member count
	err := r.db.QueryRowContext(ctx, `
		SELECT c.name, COUNT(cm.user_id)::int
		FROM classes c
		LEFT JOIN class_members cm ON cm.class_id = c.id
		WHERE c.id = $1
		GROUP BY c.name`, classID).Scan(&s.ClassName, &s.TotalMembers)
	if err != nil {
		return model.ClassStats{}, err
	}

	// 2 ── Global engagement counters (active members / last-7d / reviews / total cards)
	err = r.db.QueryRowContext(ctx, `
		SELECT
		  COUNT(DISTINCT rv.user_id)::int                                                                          AS active_members,
		  (COUNT(DISTINCT rv.user_id) FILTER (WHERE rv.updated_at >= now()-interval '7 days'))::int               AS active_last_7d,
		  (COUNT(rv.id)               FILTER (WHERE rv.updated_at >= now()-interval '7 days'))::int               AS reviews_last_7d,
		  COUNT(DISTINCT c.id)::int                                                                               AS total_cards,
		  COALESCE(
		    (COUNT(rv.id) FILTER (WHERE rv.last_result = 2))::float
		    / NULLIF(COUNT(rv.id), 0) * 100, 0
		  )                                                                                                        AS accuracy_pct
		FROM class_decks cd
		JOIN  cards   c  ON c.deck_id = cd.deck_id
		LEFT JOIN reviews rv ON rv.card_id = c.id
		WHERE cd.class_id = $1`, classID,
	).Scan(&s.ActiveMembers, &s.ActiveLast7d, &s.ReviewsLast7d, &s.TotalCards, &s.AccuracyPct)
	if err != nil {
		return model.ClassStats{}, err
	}

	// 3 ── Per-deck breakdown
	deckRows, err := r.db.QueryContext(ctx, `
		SELECT
		  d.id, d.name, d.subject,
		  COUNT(DISTINCT c.id)::int                                                              AS total_cards,
		  COUNT(DISTINCT rv.user_id)::int                                                                          AS students_studied,
		  (COUNT(DISTINCT rv.user_id) FILTER (WHERE rv.updated_at >= now()-interval '7 days'))::int               AS active_last_7d,
		  COALESCE(
		    (COUNT(rv.id) FILTER (WHERE rv.last_result = 2))::float
		    / NULLIF(COUNT(rv.id), 0) * 100, 0
		  )                                                                                                        AS accuracy_pct,
		  MAX(rv.updated_at)                                                                     AS last_activity
		FROM class_decks cd
		JOIN  decks d ON d.id = cd.deck_id
		LEFT JOIN cards   c  ON c.deck_id = d.id
		LEFT JOIN reviews rv ON rv.card_id = c.id
		WHERE cd.class_id = $1
		GROUP BY d.id, d.name, d.subject
		ORDER BY d.name ASC`, classID)
	if err != nil {
		return model.ClassStats{}, err
	}
	defer deckRows.Close()
	for deckRows.Next() {
		var ds model.ClassDeckStats
		var subj sql.NullString
		var lastAct sql.NullTime
		if err := deckRows.Scan(&ds.DeckID, &ds.DeckName, &subj,
			&ds.TotalCards, &ds.StudentsStudied, &ds.ActiveLast7d,
			&ds.AccuracyPct, &lastAct); err != nil {
			return model.ClassStats{}, err
		}
		if subj.Valid {
			ds.Subject = &subj.String
		}
		if lastAct.Valid {
			t := lastAct.Time
			ds.LastActivity = &t
		}
		s.DeckStats = append(s.DeckStats, ds)
	}
	if err := deckRows.Err(); err != nil {
		return model.ClassStats{}, err
	}

	// 4 ── Top-10 hardest cards (minimum 3 total reviews across the class)
	cardRows, err := r.db.QueryContext(ctx, `
		SELECT
		  c.id, LEFT(c.question, 120) AS question, d.name AS deck_name,
		  COUNT(rv.id)::int                                                   AS total_reviews,
		  COALESCE(
		    (COUNT(rv.id) FILTER (WHERE rv.last_result IN (0,1)))::float
		    / NULLIF(COUNT(rv.id), 0) * 100, 0
		  )                                                                   AS error_rate
		FROM class_decks cd
		JOIN  cards  c  ON c.deck_id = cd.deck_id
		JOIN  decks  d  ON d.id = c.deck_id
		JOIN  reviews rv ON rv.card_id = c.id
		WHERE cd.class_id = $1
		GROUP BY c.id, c.question, d.name
		HAVING COUNT(rv.id) >= 3
		ORDER BY error_rate DESC, total_reviews DESC
		LIMIT 10`, classID)
	if err != nil {
		return model.ClassStats{}, err
	}
	defer cardRows.Close()
	for cardRows.Next() {
		var hc model.ClassHardCard
		if err := cardRows.Scan(&hc.CardID, &hc.Question, &hc.DeckName,
			&hc.TotalReviews, &hc.ErrorRate); err != nil {
			return model.ClassStats{}, err
		}
		s.HardestCards = append(s.HardestCards, hc)
	}
	if err := cardRows.Err(); err != nil {
		return model.ClassStats{}, err
	}

	return s, nil
}

// ListClassOverview returns a compact stats summary for all classes created by a professor.
// Used on the professor dashboard to compare classes at a glance.
func (r *ClassStatsRepo) ListClassOverview(ctx context.Context, createdBy string) ([]model.ClassOverviewItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
		  c.id, c.name,
		  COUNT(DISTINCT cm.user_id)::int                                                                          AS total_members,
		  (COUNT(DISTINCT rv.user_id) FILTER (WHERE rv.updated_at >= now()-interval '7 days'))::int               AS active_last_7d,
		  (COUNT(rv.id)               FILTER (WHERE rv.updated_at >= now()-interval '7 days'))::int               AS reviews_last_7d,
		  COUNT(DISTINCT cd.deck_id)::int                                                                         AS deck_count,
		  COALESCE(
		    (COUNT(rv.id) FILTER (WHERE rv.last_result = 2))::float
		    / NULLIF(COUNT(rv.id), 0) * 100, 0
		  )                                                                                                        AS accuracy_pct,
		  MAX(rv.updated_at)                                                                                      AS last_activity
		FROM classes c
		LEFT JOIN class_members cm ON cm.class_id = c.id
		LEFT JOIN class_decks   cd ON cd.class_id = c.id
		LEFT JOIN cards          ca ON ca.deck_id  = cd.deck_id
		LEFT JOIN reviews        rv ON rv.card_id   = ca.id
		WHERE c.created_by = $1
		GROUP BY c.id, c.name
		ORDER BY c.name ASC`, createdBy)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.ClassOverviewItem
	for rows.Next() {
		var item model.ClassOverviewItem
		var lastAct sql.NullTime
		if err := rows.Scan(
			&item.ClassID, &item.ClassName,
			&item.TotalMembers, &item.ActiveLast7d, &item.ReviewsLast7d,
			&item.DeckCount, &item.AccuracyPct, &lastAct,
		); err != nil {
			return nil, err
		}
		if lastAct.Valid {
			t := lastAct.Time
			item.LastActivity = &t
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// ListAllClassOverview returns the overview for all classes (admin only).
func (r *ClassStatsRepo) ListAllClassOverview(ctx context.Context) ([]model.ClassOverviewItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
		  c.id, c.name,
		  COUNT(DISTINCT cm.user_id)::int                                                                          AS total_members,
		  (COUNT(DISTINCT rv.user_id) FILTER (WHERE rv.updated_at >= now()-interval '7 days'))::int               AS active_last_7d,
		  (COUNT(rv.id)               FILTER (WHERE rv.updated_at >= now()-interval '7 days'))::int               AS reviews_last_7d,
		  COUNT(DISTINCT cd.deck_id)::int                                                                         AS deck_count,
		  COALESCE(
		    (COUNT(rv.id) FILTER (WHERE rv.last_result = 2))::float
		    / NULLIF(COUNT(rv.id), 0) * 100, 0
		  )                                                                                                        AS accuracy_pct,
		  MAX(rv.updated_at)                                                                                      AS last_activity
		FROM classes c
		LEFT JOIN class_members cm ON cm.class_id = c.id
		LEFT JOIN class_decks   cd ON cd.class_id = c.id
		LEFT JOIN cards          ca ON ca.deck_id  = cd.deck_id
		LEFT JOIN reviews        rv ON rv.card_id   = ca.id
		GROUP BY c.id, c.name
		ORDER BY c.name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.ClassOverviewItem
	for rows.Next() {
		var item model.ClassOverviewItem
		var lastAct sql.NullTime
		if err := rows.Scan(
			&item.ClassID, &item.ClassName,
			&item.TotalMembers, &item.ActiveLast7d, &item.ReviewsLast7d,
			&item.DeckCount, &item.AccuracyPct, &lastAct,
		); err != nil {
			return nil, err
		}
		if lastAct.Valid {
			t := lastAct.Time
			item.LastActivity = &t
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
