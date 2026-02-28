package repository

import (
	"context"
	"fmt"

	"webapp/internal/model"
)

type ReviewRepo struct{ db DBTX }

func NewReviewRepo(db DBTX) *ReviewRepo { return &ReviewRepo{db: db} }

// Upsert creates or updates a review for the (user, card) pair.
func (r *ReviewRepo) Upsert(ctx context.Context, rev model.Review) (model.Review, error) {
	const q = `
		INSERT INTO reviews (user_id, card_id, next_due, last_result, streak, ease_factor, interval_days)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id, card_id) DO UPDATE
			SET next_due     = EXCLUDED.next_due,
			    last_result  = EXCLUDED.last_result,
			    streak       = EXCLUDED.streak,
			    ease_factor  = EXCLUDED.ease_factor,
			    interval_days = EXCLUDED.interval_days,
			    updated_at   = now()
		RETURNING id, user_id, card_id, next_due, last_result, streak, ease_factor, interval_days, updated_at`

	var out model.Review
	err := r.db.QueryRowContext(ctx, q,
		rev.UserID, rev.CardID, rev.NextDue, rev.LastResult, rev.Streak, rev.EaseFactor, rev.IntervalDays,
	).Scan(&out.ID, &out.UserID, &out.CardID, &out.NextDue, &out.LastResult, &out.Streak,
		&out.EaseFactor, &out.IntervalDays, &out.UpdatedAt)
	if err != nil {
		return model.Review{}, fmt.Errorf("review upsert: %w", err)
	}
	return out, nil
}

// FindByUserAndCard returns the current review record for a (user, card) pair.
func (r *ReviewRepo) FindByUserAndCard(ctx context.Context, userID, cardID string) (model.Review, error) {
	const q = `
		SELECT id, user_id, card_id, next_due, last_result, streak, ease_factor, interval_days, updated_at
		FROM reviews WHERE user_id = $1 AND card_id = $2`

	var rv model.Review
	err := r.db.QueryRowContext(ctx, q, userID, cardID).Scan(
		&rv.ID, &rv.UserID, &rv.CardID, &rv.NextDue, &rv.LastResult, &rv.Streak,
		&rv.EaseFactor, &rv.IntervalDays, &rv.UpdatedAt,
	)
	if err != nil {
		return model.Review{}, fmt.Errorf("review find: %w", err)
	}
	return rv, nil
}
