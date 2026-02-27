package repository

import (
	"context"
	"database/sql"
	"fmt"

	"webapp/internal/model"
)

type DeckRepo struct{ db DBTX }

func NewDeckRepo(db DBTX) *DeckRepo { return &DeckRepo{db: db} }

func (r *DeckRepo) Create(ctx context.Context, name string, description *string) (model.Deck, error) {
	const q = `INSERT INTO decks (name, description) VALUES ($1, $2)
	           RETURNING id, name, description, created_at`

	var d model.Deck
	var desc sql.NullString
	err := r.db.QueryRowContext(ctx, q, name, toNullString(description)).Scan(
		&d.ID, &d.Name, &desc, &d.CreatedAt,
	)
	d.Description = toStringPtr(desc)
	if err != nil {
		return model.Deck{}, fmt.Errorf("deck create: %w", err)
	}
	return d, nil
}

func (r *DeckRepo) FindByID(ctx context.Context, id string) (model.Deck, error) {
	const q = `SELECT id, name, description, created_at FROM decks WHERE id = $1`

	var d model.Deck
	var desc sql.NullString
	err := r.db.QueryRowContext(ctx, q, id).Scan(&d.ID, &d.Name, &desc, &d.CreatedAt)
	d.Description = toStringPtr(desc)
	if err != nil {
		return model.Deck{}, fmt.Errorf("deck find by id: %w", err)
	}
	return d, nil
}

// FindByName returns the deck with the given name, or sql.ErrNoRows if absent.
// Unlike FindOrCreateByName, this method never creates a new deck.
func (r *DeckRepo) FindByName(ctx context.Context, name string) (model.Deck, error) {
	const q = `SELECT id, name, description, created_at FROM decks WHERE name = $1`

	var d model.Deck
	var desc sql.NullString
	err := r.db.QueryRowContext(ctx, q, name).Scan(&d.ID, &d.Name, &desc, &d.CreatedAt)
	d.Description = toStringPtr(desc)
	if err != nil {
		return model.Deck{}, fmt.Errorf("deck find by name: %w", err)
	}
	return d, nil
}

func (r *DeckRepo) List(ctx context.Context) ([]model.Deck, error) {
	const q = `SELECT id, name, description, created_at FROM decks ORDER BY name`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("deck list: %w", err)
	}
	defer rows.Close()

	var decks []model.Deck
	for rows.Next() {
		var d model.Deck
		var desc sql.NullString
		if err := rows.Scan(&d.ID, &d.Name, &desc, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("deck scan: %w", err)
		}
		d.Description = toStringPtr(desc)
		decks = append(decks, d)
	}
	return decks, rows.Err()
}

func (r *DeckRepo) Update(ctx context.Context, id, name string, description *string) (model.Deck, error) {
	const q = `UPDATE decks SET name = $2, description = $3
	           WHERE id = $1
	           RETURNING id, name, description, created_at`

	var d model.Deck
	var desc sql.NullString
	err := r.db.QueryRowContext(ctx, q, id, name, toNullString(description)).Scan(
		&d.ID, &d.Name, &desc, &d.CreatedAt,
	)
	d.Description = toStringPtr(desc)
	if err != nil {
		return model.Deck{}, fmt.Errorf("deck update: %w", err)
	}
	return d, nil
}

func (r *DeckRepo) Delete(ctx context.Context, id string) error {
	const q = `DELETE FROM decks WHERE id = $1`
	res, err := r.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("deck delete: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// FindOrCreateByName upserts a deck by name. It returns the deck and a boolean
// wasCreated that is true when the deck did not previously exist.
//
// Internally this uses PostgreSQL's xmax trick: for a freshly inserted row
// xmax is 0 (no prior UPDATE transaction); for a row that went through the
// ON CONFLICT DO UPDATE path xmax is the current transaction ID (non-zero).
func (r *DeckRepo) FindOrCreateByName(ctx context.Context, name string) (model.Deck, bool, error) {
	const q = `
		INSERT INTO decks (name) VALUES ($1)
		ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
		RETURNING id, name, description, created_at, (xmax = 0) AS was_created`

	var d model.Deck
	var desc sql.NullString
	var wasCreated bool
	err := r.db.QueryRowContext(ctx, q, name).Scan(
		&d.ID, &d.Name, &desc, &d.CreatedAt, &wasCreated,
	)
	d.Description = toStringPtr(desc)
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
