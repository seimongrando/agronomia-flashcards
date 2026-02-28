package model

import "time"

type Role string

const (
	RoleAdmin     Role = "admin"
	RoleProfessor Role = "professor"
	RoleStudent   Role = "student"
)

func (r Role) Valid() bool {
	switch r {
	case RoleAdmin, RoleProfessor, RoleStudent:
		return true
	}
	return false
}

type User struct {
	ID         string    `json:"id"`
	GoogleSub  string    `json:"-"` // never serialise the Google identifier to JSON
	Email      string    `json:"email"`
	Name       string    `json:"name"`
	PictureURL *string   `json:"picture_url,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// MeResponse is the safe public projection of User returned by GET /api/me.
// It deliberately omits google_sub, updated_at and other internal fields.
type MeResponse struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Email      string  `json:"email"`
	PictureURL *string `json:"picture_url,omitempty"`
}

type UserRole struct {
	UserID    string    `json:"user_id"`
	Role      Role      `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}
