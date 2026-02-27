package model

import "time"

type Upload struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	DeckID        *string   `json:"deck_id,omitempty"` // nil for multi-deck imports
	Filename      string    `json:"filename"`
	ImportedCount int       `json:"imported_count"`
	UpdatedCount  int       `json:"updated_count"`
	InvalidCount  int       `json:"invalid_count"`
	DecksCreated  int       `json:"decks_created"`
	CreatedAt     time.Time `json:"created_at"`
}
