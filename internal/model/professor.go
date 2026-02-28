package model

// ProfessorStats holds aggregate data for the professor/admin dashboard.
// All values are aggregates — no individual student data is exposed (LGPD).
type ProfessorStats struct {
	TotalDecks     int        `json:"total_decks"`
	ActiveDecks    int        `json:"active_decks"`
	TotalCards     int        `json:"total_cards"`
	ActiveStudents int        `json:"active_students"` // distinct users with review in last 30 days
	TotalReviews   int        `json:"total_reviews"`
	Decks          []DeckStat `json:"decks"`
	HardestCards   []HardCard `json:"hardest_cards"` // lowest accuracy (min 5 reviews)
}

type DeckStat struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Subject          *string `json:"subject,omitempty"`
	IsActive         bool    `json:"is_active"`
	TotalCards       int     `json:"total_cards"`
	StudentsStudying int     `json:"students_studying"`
	AvgAccuracy      int     `json:"avg_accuracy"` // 0-100
	TotalReviews     int     `json:"total_reviews"`
}

type HardCard struct {
	ID           string `json:"id"`
	Question     string `json:"question"`
	Type         string `json:"type"`
	DeckName     string `json:"deck_name"`
	TotalReviews int    `json:"total_reviews"`
	Accuracy     int    `json:"accuracy"` // 0-100
}
