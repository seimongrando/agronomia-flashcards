package model

import "time"

type DeckWithCounts struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description *string    `json:"description,omitempty"`
	Subject     *string    `json:"subject,omitempty"`
	IsActive    bool       `json:"is_active"`
	IsPrivate   bool       `json:"is_private"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	CreatedBy   *string    `json:"created_by,omitempty"` // exposed to staff for UI ownership checks
	// Class context — populated on student home view when deck belongs to a class.
	ClassID     *string    `json:"class_id,omitempty"`
	ClassName   *string    `json:"class_name,omitempty"`
	TotalCards  int        `json:"total_cards"`
	DueNow      int        `json:"due_now"`
	LastStudied *time.Time `json:"last_studied,omitempty"`
	NextReview  *time.Time `json:"next_review,omitempty"`
	// Hidden is true when the student has hidden this general deck from their home page.
	// Only populated on student-facing list responses (ApplyVisibility=true).
	Hidden bool `json:"hidden,omitempty"`
}

// HideDeckRequest is the body for POST /api/me/deck-hidden.
type HideDeckRequest struct {
	DeckID string `json:"deck_id"`
	Hidden bool   `json:"hidden"`
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

// OfflineBundle is returned by GET /api/study/offline and contains everything
// a browser needs to study a deck without network access.
type OfflineBundle struct {
	Cards   []Card                   `json:"cards"`
	Reviews map[string]OfflineReview `json:"reviews"` // cardID → review state
}

// OfflineReview is the minimal review state needed to run SM-2 offline.
type OfflineReview struct {
	Streak       int     `json:"streak"`
	IntervalDays int     `json:"interval_days"`
	EaseFactor   float64 `json:"ease_factor"`
	NextDue      string  `json:"next_due"` // RFC3339
	LastResult   int16   `json:"last_result"`
	UpdatedAt    string  `json:"updated_at"` // RFC3339 — used to sort "wrong" mode
}

type StudyStats struct {
	DueNow        int `json:"due_now"`
	ReviewedToday int `json:"reviewed_today"`
	AccuracyPct   int `json:"accuracy_pct"`
	TotalCards    int `json:"total_cards"` // all cards in this deck (for session progress)
}
