package model

import "time"

type Deck struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	Subject     *string   `json:"subject,omitempty"` // optional discipline grouping
	CreatedAt   time.Time `json:"created_at"`
}
