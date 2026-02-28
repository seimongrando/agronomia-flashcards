package model

import "time"

type DeckWithCounts struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description *string    `json:"description,omitempty"`
	Subject     *string    `json:"subject,omitempty"`
	IsActive    bool       `json:"is_active"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	CreatedBy   *string    `json:"created_by,omitempty"` // exposed to staff for UI ownership checks
	TotalCards  int        `json:"total_cards"`
	DueNow      int        `json:"due_now"`
	LastStudied *time.Time `json:"last_studied,omitempty"`
	NextReview  *time.Time `json:"next_review,omitempty"`
}

type AnswerRequest struct {
	CardID string `json:"card_id"`
	Result int    `json:"result"` // 0=wrong, 1=hard, 2=correct
}

type AnswerResponse struct {
	NextDue      time.Time `json:"next_due"`
	Streak       int       `json:"streak"`
	IntervalDays int       `json:"interval_days"`
}

type StudyStats struct {
	DueNow        int `json:"due_now"`
	ReviewedToday int `json:"reviewed_today"`
	AccuracyPct   int `json:"accuracy_pct"`
	TotalCards    int `json:"total_cards"` // all cards in this deck (for session progress)
}
