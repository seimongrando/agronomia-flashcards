package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"webapp/internal/model"
)

type DeckRepo struct{ db DBTX }

func NewDeckRepo(db DBTX) *DeckRepo { return &DeckRepo{db: db} }

const deckCols = `id, name, description, subject, is_active, expires_at, created_at`

func scanDeck(row interface {
	Scan(dest ...any) error
}) (model.Deck, error) {
	var d model.Deck
	var desc, subj sql.NullString
	var exp sql.NullTime
	if err := row.Scan(&d.ID, &d.Name, &desc, &subj, &d.IsActive, &exp, &d.CreatedAt); err != nil {
		return model.Deck{}, err
	}
	d.Description = toStringPtr(desc)
	d.Subject = toStringPtr(subj)
	if exp.Valid {
		t := exp.Time
		d.ExpiresAt = &t
	}
	return d, nil
}

func (r *DeckRepo) Create(ctx context.Context, name string, description, subject *string) (model.Deck, error) {
	q := `INSERT INTO decks (name, description, subject) VALUES ($1, $2, $3) RETURNING ` + deckCols
	d, err := scanDeck(r.db.QueryRowContext(ctx, q, name, toNullString(description), toNullString(subject)))
	if err != nil {
		return model.Deck{}, fmt.Errorf("deck create: %w", err)
	}
	return d, nil
}

func (r *DeckRepo) FindByID(ctx context.Context, id string) (model.Deck, error) {
	q := `SELECT ` + deckCols + ` FROM decks WHERE id = $1`
	d, err := scanDeck(r.db.QueryRowContext(ctx, q, id))
	if err != nil {
		return model.Deck{}, fmt.Errorf("deck find by id: %w", err)
	}
	return d, nil
}

func (r *DeckRepo) FindByName(ctx context.Context, name string) (model.Deck, error) {
	q := `SELECT ` + deckCols + ` FROM decks WHERE name = $1`
	d, err := scanDeck(r.db.QueryRowContext(ctx, q, name))
	if err != nil {
		return model.Deck{}, fmt.Errorf("deck find by name: %w", err)
	}
	return d, nil
}

func (r *DeckRepo) List(ctx context.Context) ([]model.Deck, error) {
	q := `SELECT ` + deckCols + ` FROM decks ORDER BY subject NULLS LAST, name`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("deck list: %w", err)
	}
	defer rows.Close()

	var decks []model.Deck
	for rows.Next() {
		d, err := scanDeck(rows)
		if err != nil {
			return nil, fmt.Errorf("deck scan: %w", err)
		}
		decks = append(decks, d)
	}
	return decks, rows.Err()
}

func (r *DeckRepo) Update(ctx context.Context, id, name string, description, subject *string) (model.Deck, error) {
	q := `UPDATE decks SET name=$2, description=$3, subject=$4
	      WHERE id=$1 RETURNING ` + deckCols
	d, err := scanDeck(r.db.QueryRowContext(ctx, q, id, name, toNullString(description), toNullString(subject)))
	if err != nil {
		return model.Deck{}, fmt.Errorf("deck update: %w", err)
	}
	return d, nil
}

// Patch updates is_active and/or expires_at.
// isActive: nil = keep current; non-nil = set to that value.
// clearExpiry: true = set expires_at to NULL, ignoring expiresAt.
// expiresAt: non-nil = set to that timestamp (only when clearExpiry is false).
func (r *DeckRepo) Patch(ctx context.Context, id string, isActive *bool, expiresAt *time.Time, clearExpiry bool) (model.Deck, error) {
	// Read current values first so we can do a full UPDATE without losing fields.
	cur, err := r.FindByID(ctx, id)
	if err != nil {
		return model.Deck{}, fmt.Errorf("deck patch: %w", err)
	}

	newActive := cur.IsActive
	if isActive != nil {
		newActive = *isActive
	}

	var newExpires *time.Time
	if !clearExpiry {
		if expiresAt != nil {
			newExpires = expiresAt
		} else {
			newExpires = cur.ExpiresAt // keep existing
		}
	}
	// clearExpiry=true → newExpires stays nil → column set to NULL

	q := `UPDATE decks SET is_active = $2, expires_at = $3
	      WHERE id = $1
	      RETURNING ` + deckCols

	var expArg interface{}
	if newExpires != nil {
		expArg = *newExpires
	}

	d, err := scanDeck(r.db.QueryRowContext(ctx, q, id, newActive, expArg))
	if err != nil {
		return model.Deck{}, fmt.Errorf("deck patch: %w", err)
	}
	return d, nil
}

func (r *DeckRepo) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM decks WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deck delete: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// FindOrCreateByName upserts a deck by name (used during CSV import).
// Subject and active status are not touched — set separately by the professor.
func (r *DeckRepo) FindOrCreateByName(ctx context.Context, name string) (model.Deck, bool, error) {
	q := `INSERT INTO decks (name) VALUES ($1)
		  ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
		  RETURNING ` + deckCols + `, (xmax = 0) AS was_created`

	var d model.Deck
	var desc, subj sql.NullString
	var exp sql.NullTime
	var wasCreated bool
	err := r.db.QueryRowContext(ctx, q, name).Scan(
		&d.ID, &d.Name, &desc, &subj, &d.IsActive, &exp, &d.CreatedAt, &wasCreated,
	)
	d.Description = toStringPtr(desc)
	d.Subject = toStringPtr(subj)
	if exp.Valid {
		t := exp.Time
		d.ExpiresAt = &t
	}
	if err != nil {
		return model.Deck{}, false, fmt.Errorf("deck find or create: %w", err)
	}
	return d, wasCreated, nil
}

func (r *DeckRepo) CardCount(ctx context.Context, id string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cards WHERE deck_id = $1`, id).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("deck card count: %w", err)
	}
	return count, nil
}
