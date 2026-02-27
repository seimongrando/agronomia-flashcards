package model

import "time"

const (
	MaxQuestionLen = 500
	MaxAnswerLen   = 2000
	MaxTopicLen    = 80
	MaxSourceLen   = 120
	MaxDeckNameLen = 80
)

type CardType string

const (
	CardTypeConceito   CardType = "conceito"
	CardTypeProcesso   CardType = "processo"
	CardTypeAplicacao  CardType = "aplicacao"
	CardTypeComparacao CardType = "comparacao"
)

func (t CardType) Valid() bool {
	switch t {
	case CardTypeConceito, CardTypeProcesso, CardTypeAplicacao, CardTypeComparacao:
		return true
	}
	return false
}

type Card struct {
	ID        string    `json:"id"`
	DeckID    string    `json:"deck_id"`
	Topic     *string   `json:"topic,omitempty"`
	Type      CardType  `json:"type"`
	Question  string    `json:"question"`
	Answer    string    `json:"answer"`
	Source    *string   `json:"source,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CardListItem is the reduced projection returned by the list endpoint.
// Answer and Source are intentionally omitted to minimise payload and protect
// study integrity (answer visible only when a card is opened for editing).
// Use GET /api/content/cards/:id for the full card.
type CardListItem struct {
	ID        string    `json:"id"`
	DeckID    string    `json:"deck_id"`
	Type      CardType  `json:"type"`
	Topic     *string   `json:"topic,omitempty"`
	Question  string    `json:"question"`
	UpdatedAt time.Time `json:"updated_at"`
}
