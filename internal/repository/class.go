package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"fmt"
	"strings"
	"time"

	"webapp/internal/model"
)

type ClassRepo struct{ db DBTX }

func NewClassRepo(db DBTX) *ClassRepo { return &ClassRepo{db: db} }

// generateInviteCode creates a random 8-char uppercase alphanumeric code.
func generateInviteCode() (string, error) {
	b := make([]byte, 5)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	code := strings.TrimRight(base32.StdEncoding.EncodeToString(b), "=")
	if len(code) > 8 {
		code = code[:8]
	}
	return code, nil
}

// classWithCountQuery wraps any class query to include the live member count.
const classWithCountQuery = `
	SELECT c.id, c.name, c.description, c.invite_code, c.is_active,
	       COUNT(cm.user_id)::int AS member_count,
	       c.created_by, c.created_at, c.updated_at
	FROM classes c
	LEFT JOIN class_members cm ON cm.class_id = c.id`

func scanClass(row interface{ Scan(...any) error }) (model.Class, error) {
	var c model.Class
	var desc sql.NullString
	err := row.Scan(&c.ID, &c.Name, &desc, &c.InviteCode, &c.IsActive, &c.MemberCount, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt)
	if desc.Valid {
		c.Description = &desc.String
	}
	return c, err
}

func (r *ClassRepo) Create(ctx context.Context, name string, description *string, createdBy string) (model.Class, error) {
	code, err := generateInviteCode()
	if err != nil {
		return model.Class{}, fmt.Errorf("invite code: %w", err)
	}
	// Insert first, then re-fetch with member count (0 for new class).
	const q = `INSERT INTO classes (name, description, invite_code, created_by) VALUES ($1, $2, $3, $4) RETURNING id`
	var id string
	if err := r.db.QueryRowContext(ctx, q, name, toNullString(description), code, createdBy).Scan(&id); err != nil {
		return model.Class{}, fmt.Errorf("class create: %w", err)
	}
	return r.FindByID(ctx, id)
}

func (r *ClassRepo) FindByID(ctx context.Context, id string) (model.Class, error) {
	q := classWithCountQuery + ` WHERE c.id = $1 GROUP BY c.id`
	return scanClass(r.db.QueryRowContext(ctx, q, id))
}

func (r *ClassRepo) FindByInviteCode(ctx context.Context, code string) (model.Class, error) {
	q := classWithCountQuery + ` WHERE c.invite_code = $1 AND c.is_active = true GROUP BY c.id`
	return scanClass(r.db.QueryRowContext(ctx, q, code))
}

func (r *ClassRepo) Update(ctx context.Context, id, name string, description *string, isActive bool) (model.Class, error) {
	_, err := r.db.ExecContext(ctx,
		`UPDATE classes SET name=$2, description=$3, is_active=$4, updated_at=now() WHERE id=$1`,
		id, name, toNullString(description), isActive)
	if err != nil {
		return model.Class{}, err
	}
	return r.FindByID(ctx, id)
}

func (r *ClassRepo) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM classes WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// RegenerateInviteCode issues a fresh invite code for a class.
func (r *ClassRepo) RegenerateInviteCode(ctx context.Context, id string) (string, error) {
	code, err := generateInviteCode()
	if err != nil {
		return "", err
	}
	_, err = r.db.ExecContext(ctx,
		`UPDATE classes SET invite_code=$2, updated_at=now() WHERE id=$1`, id, code)
	return code, err
}

// ListByCreator returns all classes created by a professor/admin (includes invite_code).
func (r *ClassRepo) ListByCreator(ctx context.Context, userID string) ([]model.ClassSummary, error) {
	const q = `
		SELECT c.id, c.name, c.description, c.invite_code, c.is_active,
		       COUNT(DISTINCT cd.deck_id)::int AS deck_count,
		       COUNT(DISTINCT cm.user_id)::int  AS member_count
		FROM classes c
		LEFT JOIN class_decks   cd ON cd.class_id = c.id
		LEFT JOIN class_members cm ON cm.class_id = c.id
		WHERE c.created_by = $1
		GROUP BY c.id, c.name, c.description, c.invite_code, c.is_active
		ORDER BY c.name ASC
		LIMIT 500`
	rows, err := r.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ClassSummary
	for rows.Next() {
		var s model.ClassSummary
		var desc sql.NullString
		var code string
		if err := rows.Scan(&s.ID, &s.Name, &desc, &code, &s.IsActive, &s.DeckCount, &s.MemberCount); err != nil {
			return nil, err
		}
		if desc.Valid {
			s.Description = &desc.String
		}
		s.InviteCode = &code
		out = append(out, s)
	}
	return out, rows.Err()
}

// ListByMember returns active classes a student is enrolled in (no invite_code).
func (r *ClassRepo) ListByMember(ctx context.Context, userID string) ([]model.ClassSummary, error) {
	const q = `
		SELECT c.id, c.name, c.description, c.is_active,
		       COUNT(DISTINCT cd.deck_id)::int  AS deck_count,
		       COUNT(DISTINCT cm2.user_id)::int AS member_count,
		       MAX(cm.joined_at) AS joined_at
		FROM classes c
		JOIN  class_members cm  ON cm.class_id = c.id AND cm.user_id = $1
		LEFT JOIN class_decks   cd  ON cd.class_id = c.id
		LEFT JOIN class_members cm2 ON cm2.class_id = c.id
		WHERE c.is_active = true
		GROUP BY c.id, c.name, c.description, c.is_active
		ORDER BY c.name ASC
		LIMIT 200`
	rows, err := r.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ClassSummary
	for rows.Next() {
		var s model.ClassSummary
		var desc sql.NullString
		var joinedAt time.Time
		if err := rows.Scan(&s.ID, &s.Name, &desc, &s.IsActive, &s.DeckCount, &s.MemberCount, &joinedAt); err != nil {
			return nil, err
		}
		if desc.Valid {
			s.Description = &desc.String
		}
		s.JoinedAt = &joinedAt
		out = append(out, s)
	}
	return out, rows.Err()
}

// AddDeck assigns a deck to a class. Idempotent.
func (r *ClassRepo) AddDeck(ctx context.Context, classID, deckID string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO class_decks (class_id, deck_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		classID, deckID)
	return err
}

// RemoveDeck unassigns a deck from a class.
func (r *ClassRepo) RemoveDeck(ctx context.Context, classID, deckID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM class_decks WHERE class_id=$1 AND deck_id=$2`, classID, deckID)
	return err
}

// IsMember reports whether userID is enrolled in classID.
func (r *ClassRepo) IsMember(ctx context.Context, classID, userID string) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM class_members WHERE class_id=$1 AND user_id=$2`, classID, userID).Scan(&n)
	return n > 0, err
}

// AddMember enrolls a student. Idempotent.
func (r *ClassRepo) AddMember(ctx context.Context, classID, userID string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO class_members (class_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		classID, userID)
	return err
}

// RemoveMember removes a student from a class (used by student to leave).
func (r *ClassRepo) RemoveMember(ctx context.Context, classID, userID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM class_members WHERE class_id=$1 AND user_id=$2`, classID, userID)
	return err
}

// ListClassDecks returns decks assigned to a class with their card counts.
func (r *ClassRepo) ListClassDecks(ctx context.Context, classID string) ([]model.ClassDeckSummary, error) {
	const q = `
		SELECT d.id, d.name, d.subject, COUNT(c.id)::int AS card_count, cd.added_at
		FROM class_decks cd
		JOIN  decks d ON d.id = cd.deck_id
		LEFT JOIN cards c ON c.deck_id = d.id
		WHERE cd.class_id = $1
		GROUP BY d.id, d.name, d.subject, cd.added_at
		ORDER BY d.name ASC
		LIMIT 500`
	rows, err := r.db.QueryContext(ctx, q, classID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ClassDeckSummary
	for rows.Next() {
		var s model.ClassDeckSummary
		var subj sql.NullString
		if err := rows.Scan(&s.DeckID, &s.DeckName, &subj, &s.CardCount, &s.AddedAt); err != nil {
			return nil, err
		}
		if subj.Valid {
			s.Subject = &subj.String
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
