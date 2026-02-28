package model

import "time"

type Deck struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description *string    `json:"description,omitempty"`
	Subject     *string    `json:"subject,omitempty"`
	IsActive    bool       `json:"is_active"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	CreatedBy   *string    `json:"created_by,omitempty"` // professor/admin who owns this deck
}

// IsOwnedBy returns true if the deck was created by the given userID.
// Decks with no owner (NULL created_by, i.e. pre-migration) return false.
func (d Deck) IsOwnedBy(userID string) bool {
	return d.CreatedBy != nil && *d.CreatedBy == userID
}

// EffectivelyActive returns true when the deck is enabled and not past its expiry date.
func (d Deck) EffectivelyActive() bool {
	if !d.IsActive {
		return false
	}
	if d.ExpiresAt != nil && d.ExpiresAt.Before(time.Now()) {
		return false
	}
	return true
}
