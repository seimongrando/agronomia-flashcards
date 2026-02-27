package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"

	"webapp/internal/model"
)

type UserRepo struct{ db DBTX }

func NewUserRepo(db DBTX) *UserRepo { return &UserRepo{db: db} }

func (r *UserRepo) UpsertByGoogleSub(ctx context.Context, u model.User) (model.User, error) {
	const q = `
		INSERT INTO users (google_sub, email, name, picture_url)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (google_sub) DO UPDATE
			SET email       = EXCLUDED.email,
			    name        = EXCLUDED.name,
			    picture_url = EXCLUDED.picture_url,
			    updated_at  = now()
		RETURNING id, google_sub, email, name, picture_url, created_at, updated_at`

	var out model.User
	var pic sql.NullString
	err := r.db.QueryRowContext(ctx, q,
		u.GoogleSub, u.Email, u.Name, toNullString(u.PictureURL),
	).Scan(&out.ID, &out.GoogleSub, &out.Email, &out.Name, &pic, &out.CreatedAt, &out.UpdatedAt)
	out.PictureURL = toStringPtr(pic)
	if err != nil {
		return model.User{}, fmt.Errorf("user upsert: %w", err)
	}
	return out, nil
}

func (r *UserRepo) FindByID(ctx context.Context, id string) (model.User, error) {
	const q = `SELECT id, google_sub, email, name, picture_url, created_at, updated_at
	           FROM users WHERE id = $1`

	var u model.User
	var pic sql.NullString
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&u.ID, &u.GoogleSub, &u.Email, &u.Name, &pic, &u.CreatedAt, &u.UpdatedAt,
	)
	u.PictureURL = toStringPtr(pic)
	if err != nil {
		return model.User{}, fmt.Errorf("user find by id: %w", err)
	}
	return u, nil
}

func (r *UserRepo) FindByGoogleSub(ctx context.Context, sub string) (model.User, error) {
	const q = `SELECT id, google_sub, email, name, picture_url, created_at, updated_at
	           FROM users WHERE google_sub = $1`

	var u model.User
	var pic sql.NullString
	err := r.db.QueryRowContext(ctx, q, sub).Scan(
		&u.ID, &u.GoogleSub, &u.Email, &u.Name, &pic, &u.CreatedAt, &u.UpdatedAt,
	)
	u.PictureURL = toStringPtr(pic)
	if err != nil {
		return model.User{}, fmt.Errorf("user find by google_sub: %w", err)
	}
	return u, nil
}

func (r *UserRepo) AddRole(ctx context.Context, userID string, role model.Role) error {
	const q = `INSERT INTO user_roles (user_id, role) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	_, err := r.db.ExecContext(ctx, q, userID, string(role))
	if err != nil {
		return fmt.Errorf("add role: %w", err)
	}
	return nil
}

func (r *UserRepo) RolesByUserID(ctx context.Context, userID string) ([]model.Role, error) {
	const q = `SELECT role FROM user_roles WHERE user_id = $1 ORDER BY role`
	rows, err := r.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	defer rows.Close()

	var roles []model.Role
	for rows.Next() {
		var role model.Role
		if err := rows.Scan(&role); err != nil {
			return nil, fmt.Errorf("scan role: %w", err)
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (r *UserRepo) RemoveRole(ctx context.Context, userID string, role model.Role) error {
	const q = `DELETE FROM user_roles WHERE user_id = $1 AND role = $2`
	res, err := r.db.ExecContext(ctx, q, userID, string(role))
	if err != nil {
		return fmt.Errorf("remove role: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// UserListParams parameterises ListUsersWithRolesPaged.
type UserListParams struct {
	EmailQuery string
	CursorTS   time.Time // zero = first page
	CursorID   string
	Limit      int
}

// ListUsersWithRolesPaged returns a page of users ordered by (created_at DESC, id DESC).
// google_sub and picture_url are excluded (data minimisation / LGPD).
// It fetches Limit+1 rows so the caller can detect whether a next page exists.
func (r *UserRepo) ListUsersWithRolesPaged(ctx context.Context, p UserListParams) ([]model.UserWithRoles, error) {
	var sb strings.Builder
	var args []any

	nextArg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	sb.WriteString(`
		SELECT u.id, u.email, u.name, u.created_at,
		       COALESCE(array_agg(ur.role ORDER BY ur.role)
		                FILTER (WHERE ur.role IS NOT NULL), '{}') AS roles
		FROM users u
		LEFT JOIN user_roles ur ON ur.user_id = u.id
		WHERE 1=1`)

	if p.EmailQuery != "" {
		ph := nextArg(p.EmailQuery)
		fmt.Fprintf(&sb, ` AND u.email ILIKE '%%' || %s || '%%'`, ph)
	}

	if !p.CursorTS.IsZero() {
		tsArg := nextArg(p.CursorTS)
		idArg := nextArg(p.CursorID)
		fmt.Fprintf(&sb, ` AND (u.created_at, u.id) < (%s::timestamptz, %s::uuid)`, tsArg, idArg)
	}

	sb.WriteString(` GROUP BY u.id, u.email, u.name, u.created_at`)
	sb.WriteString(` ORDER BY u.created_at DESC, u.id DESC LIMIT `)
	sb.WriteString(nextArg(p.Limit))

	rows, err := r.db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []model.UserWithRoles
	for rows.Next() {
		var u model.UserWithRoles
		var roles pq.StringArray
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.CreatedAt, &roles); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		u.Roles = roles
		users = append(users, u)
	}
	return users, rows.Err()
}

// ListUsersWithRoles is kept for backward compatibility (used by admin_test).
func (r *UserRepo) ListUsersWithRoles(ctx context.Context, emailQuery string) ([]model.UserWithRoles, error) {
	return r.ListUsersWithRolesPaged(ctx, UserListParams{
		EmailQuery: emailQuery,
		Limit:      50,
	})
}
