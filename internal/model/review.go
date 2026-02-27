package model

import "time"

type Review struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	CardID       string    `json:"card_id"`
	NextDue      time.Time `json:"next_due"`
	LastResult   int16     `json:"last_result"`
	Streak       int       `json:"streak"`
	EaseFactor   float64   `json:"ease_factor"`   // SM-2 inter-repetition multiplier (≥1.3)
	IntervalDays int       `json:"interval_days"` // last scheduled interval in days
	UpdatedAt    time.Time `json:"updated_at"`
}
