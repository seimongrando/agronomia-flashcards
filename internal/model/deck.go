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
