package model

type CreateDeckRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

type UpdateDeckRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
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
