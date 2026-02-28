package model

type CreateDeckRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Subject     *string `json:"subject"` // optional discipline (e.g. "Química do Solo")
}

type UpdateDeckRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Subject     *string `json:"subject"`
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
