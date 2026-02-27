package model

import "time"

// UserWithRoles is the response shape for admin user listings.
// google_sub and picture_url are intentionally excluded (data minimisation).
type UserWithRoles struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Roles     []string  `json:"roles"`
	CreatedAt time.Time `json:"created_at"`
}

type SetRolesRequest struct {
	Add    []string `json:"add"`
	Remove []string `json:"remove"`
}
