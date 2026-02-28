package model

// ProgressStats is the payload returned by GET /api/progress.
type ProgressStats struct {
	TotalStudied  int            `json:"total_studied"`  // cards with at least 1 review
	Mastered      int            `json:"mastered"`       // streak >= 3
	Learning      int            `json:"learning"`       // 0 < streak < 3
	DueToday      int            `json:"due_today"`      // next_due <= now()
	Accuracy7d    int            `json:"accuracy_7d"`    // % correct in last 7 days (0-100)
	StudyDays     int            `json:"study_days"`     // distinct calendar days with any review
	StudyStreak   int            `json:"study_streak"`   // current consecutive-day streak
	LongestStreak int            `json:"longest_streak"` // all-time longest streak
	Decks         []DeckProgress `json:"decks"`
}

// DeckProgress is the per-deck breakdown inside ProgressStats.
type DeckProgress struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	TotalCards int    `json:"total_cards"`
	Mastered   int    `json:"mastered"`
	Learning   int    `json:"learning"`
	DueNow     int    `json:"due_now"`
	Wrong      int    `json:"wrong"` // cards whose last answer was "errei" (last_result = 0)
	Hard       int    `json:"hard"`  // cards whose last answer was "difícil" (last_result = 1)
}
