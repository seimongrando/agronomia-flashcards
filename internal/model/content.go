package model

type CreateDeckRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Subject     *string `json:"subject"`
}

type UpdateDeckRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Subject     *string `json:"subject"`
}

// PatchDeckRequest allows partial updates — only provided (non-nil) fields are applied.
type PatchDeckRequest struct {
	IsActive  *bool   `json:"is_active"`
	ExpiresAt *string `json:"expires_at"` // RFC3339 string or "" to clear
}

type CreateCardRequest struct {
	DeckID   string  `json:"deck_id"`
	Topic    *string `json:"topic"`
	Type     string  `json:"type"`
	Question string  `json:"question"`
	Answer   string  `json:"answer"`
	Source   *string `json:"source"`
}

type UpdateCardRequest struct {
	Topic    *string `json:"topic"`
	Type     string  `json:"type"`
	Question string  `json:"question"`
	Answer   string  `json:"answer"`
	Source   *string `json:"source"`
}

type ImportResult struct {
	ImportedCount int `json:"imported_count"`
	UpdatedCount  int `json:"updated_count"`
	InvalidCount  int `json:"invalid_count"`
	DecksCreated  int `json:"decks_created"`
}
