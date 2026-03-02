package repository

import (
	"context"
	"database/sql"
	"fmt"

	"webapp/internal/model"
)

type UploadRepo struct{ db DBTX }

func NewUploadRepo(db DBTX) *UploadRepo { return &UploadRepo{db: db} }

func (r *UploadRepo) Create(ctx context.Context, u model.Upload) (model.Upload, error) {
	const q = `
		INSERT INTO uploads
		            (user_id, deck_id, filename, imported_count, updated_count, invalid_count, decks_created)
		VALUES      ($1, $2, $3, $4, $5, $6, $7)
		RETURNING   id, user_id, deck_id, filename,
		            imported_count, updated_count, invalid_count, decks_created, created_at`

	var out model.Upload
	var deckID sql.NullString
	err := r.db.QueryRowContext(ctx, q,
		u.UserID, toNullString(u.DeckID), u.Filename,
		u.ImportedCount, u.UpdatedCount, u.InvalidCount, u.DecksCreated,
	).Scan(
		&out.ID, &out.UserID, &deckID, &out.Filename,
		&out.ImportedCount, &out.UpdatedCount, &out.InvalidCount, &out.DecksCreated,
		&out.CreatedAt,
	)
	out.DeckID = toStringPtr(deckID)
	if err != nil {
		return model.Upload{}, fmt.Errorf("upload create: %w", err)
	}
	return out, nil
}

func (r *UploadRepo) ListByDeck(ctx context.Context, deckID string) ([]model.Upload, error) {
	const q = `
		SELECT id, user_id, deck_id, filename,
		       imported_count, updated_count, invalid_count, decks_created, created_at
		FROM uploads WHERE deck_id = $1
		ORDER BY created_at DESC
		LIMIT 200`

	rows, err := r.db.QueryContext(ctx, q, deckID)
	if err != nil {
		return nil, fmt.Errorf("upload list: %w", err)
	}
	defer rows.Close()

	var uploads []model.Upload
	for rows.Next() {
		var u model.Upload
		var dk sql.NullString
		if err := rows.Scan(
			&u.ID, &u.UserID, &dk, &u.Filename,
			&u.ImportedCount, &u.UpdatedCount, &u.InvalidCount, &u.DecksCreated,
			&u.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("upload scan: %w", err)
		}
		u.DeckID = toStringPtr(dk)
		uploads = append(uploads, u)
	}
	return uploads, rows.Err()
}
