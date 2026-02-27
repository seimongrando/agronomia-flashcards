package model

import "time"

type DeckWithCounts struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description *string    `json:"description,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	TotalCards  int        `json:"total_cards"`
	DueNow      int        `json:"due_now"`
	LastStudied *time.Time `json:"last_studied,omitempty"` // latest review by this user for this deck
	NextReview  *time.Time `json:"next_review,omitempty"`  // earliest future next_due for this user+deck
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
