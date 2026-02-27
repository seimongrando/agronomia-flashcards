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
	GoogleSub  string    `json:"google_sub"`
	Email      string    `json:"email"`
	Name       string    `json:"name"`
	PictureURL *string   `json:"picture_url,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type UserRole struct {
	UserID    string    `json:"user_id"`
	Role      Role      `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}
