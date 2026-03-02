package model

import "time"

// Class is the full record (used internally and returned to deck owners).
type Class struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	InviteCode  string    `json:"invite_code,omitempty"`
	IsActive    bool      `json:"is_active"`
	MemberCount int       `json:"member_count"` // enrolled student count
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	CreatedBy   string    `json:"-"` // never serialized — internal ownership check
}

// ClassSummary is the list-view DTO.
// InviteCode is only populated for professor/admin views (data minimization).
// JoinedAt is only populated for student views.
type ClassSummary struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description *string    `json:"description,omitempty"`
	InviteCode  *string    `json:"invite_code,omitempty"`
	DeckCount   int        `json:"deck_count"`
	MemberCount int        `json:"member_count"`
	IsActive    bool       `json:"is_active"`
	JoinedAt    *time.Time `json:"joined_at,omitempty"`
}

// ClassDeckSummary is a deck as listed within a class management page.
type ClassDeckSummary struct {
	DeckID    string    `json:"deck_id"`
	DeckName  string    `json:"deck_name"`
	Subject   *string   `json:"subject,omitempty"`
	CardCount int       `json:"card_count"`
	AddedAt   time.Time `json:"added_at"`
}
