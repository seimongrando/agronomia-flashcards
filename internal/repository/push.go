package repository

import (
	"context"

	"webapp/internal/model"
)

type PushRepo struct{ db DBTX }

func NewPushRepo(db DBTX) *PushRepo { return &PushRepo{db: db} }

// Upsert saves a push subscription; idempotent on (user_id, endpoint).
func (r *PushRepo) Upsert(ctx context.Context, sub model.PushSubscription) error {
	const q = `
		INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, endpoint) DO UPDATE
			SET p256dh = EXCLUDED.p256dh,
			    auth   = EXCLUDED.auth`
	_, err := r.db.ExecContext(ctx, q, sub.UserID, sub.Endpoint, sub.P256DH, sub.Auth)
	return err
}

// Delete removes a specific subscription (e.g. when the browser unsubscribes).
func (r *PushRepo) Delete(ctx context.Context, userID, endpoint string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM push_subscriptions WHERE user_id = $1 AND endpoint = $2`,
		userID, endpoint)
	return err
}

// DeleteGone removes a subscription whose endpoint returned 410 Gone.
func (r *PushRepo) DeleteGone(ctx context.Context, endpoint string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM push_subscriptions WHERE endpoint = $1`, endpoint)
	return err
}

// ListWithDueCards returns all subscriptions for users who have cards due today.
// DueCount is the number of due reviews. Used by the daily notification scheduler.
func (r *PushRepo) ListWithDueCards(ctx context.Context) ([]model.PushSubWithDue, error) {
	const q = `
		SELECT ps.user_id, ps.endpoint, ps.p256dh, ps.auth,
		       COUNT(rv.card_id)::int AS due_count
		FROM push_subscriptions ps
		JOIN reviews rv ON rv.user_id = ps.user_id AND rv.next_due <= now()
		GROUP BY ps.user_id, ps.endpoint, ps.p256dh, ps.auth
		HAVING COUNT(rv.card_id) > 0
		LIMIT 2000`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.PushSubWithDue
	for rows.Next() {
		var s model.PushSubWithDue
		if err := rows.Scan(&s.UserID, &s.Endpoint, &s.P256DH, &s.Auth, &s.DueCount); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
