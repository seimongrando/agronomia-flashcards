package model

import "time"

// ClassStats is the full report for a single class (turma).
// All figures are aggregates — no individual student data is exposed (LGPD).
type ClassStats struct {
	ClassID       string  `json:"class_id"`
	ClassName     string  `json:"class_name"`
	TotalMembers  int     `json:"total_members"`
	ActiveMembers int     `json:"active_members"`  // ever studied at least one deck card
	ActiveLast7d  int     `json:"active_last_7d"`  // studied in the past 7 days
	ReviewsLast7d int     `json:"reviews_last_7d"` // total review events in past 7 days
	TotalCards    int     `json:"total_cards"`     // total cards across all class decks
	AccuracyPct   float64 `json:"accuracy_pct"`    // overall % of correct answers

	DeckStats    []ClassDeckStats `json:"deck_stats"`
	HardestCards []ClassHardCard  `json:"hardest_cards"`
}

// ClassDeckStats breaks down a single deck's performance within a class.
type ClassDeckStats struct {
	DeckID          string     `json:"deck_id"`
	DeckName        string     `json:"deck_name"`
	Subject         *string    `json:"subject,omitempty"`
	TotalCards      int        `json:"total_cards"`
	StudentsStudied int        `json:"students_studied"` // distinct students who reviewed ≥1 card
	ActiveLast7d    int        `json:"active_last_7d"`   // distinct students active in past 7d
	AccuracyPct     float64    `json:"accuracy_pct"`
	LastActivity    *time.Time `json:"last_activity,omitempty"`
}

// ClassHardCard surfaces the most problematic cards in a class (highest error rate).
type ClassHardCard struct {
	CardID       string  `json:"card_id"`
	Question     string  `json:"question"`
	DeckName     string  `json:"deck_name"`
	ErrorRate    float64 `json:"error_rate"`    // % of wrong+hard answers
	TotalReviews int     `json:"total_reviews"` // total attempts by all students
}

// ClassOverviewItem is a compact summary of a class shown on the professor dashboard.
type ClassOverviewItem struct {
	ClassID       string     `json:"class_id"`
	ClassName     string     `json:"class_name"`
	TotalMembers  int        `json:"total_members"`
	ActiveLast7d  int        `json:"active_last_7d"`
	ReviewsLast7d int        `json:"reviews_last_7d"`
	DeckCount     int        `json:"deck_count"`
	AccuracyPct   float64    `json:"accuracy_pct"`
	LastActivity  *time.Time `json:"last_activity,omitempty"`
}
