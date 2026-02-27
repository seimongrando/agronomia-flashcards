package csvparse

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"webapp/internal/model"
)

const (
	MaxFileSize    = 2 << 20 // 2 MB
	MaxRows        = 2000
	MaxPreviewRows = 50
)

// ParseOptions controls optional parser behaviour.
type ParseOptions struct {
	// DefaultDeck is used when the CSV has no "deck" column (single-deck mode).
	// If the CSV contains a "deck" column this field is ignored.
	// If the CSV has no "deck" column and DefaultDeck is empty, Parse returns an error.
	DefaultDeck string
}

// Row represents one data row after parsing and validation.
type Row struct {
	Line     int    `json:"line"`
	Deck     string `json:"deck"`
	Type     string `json:"type"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
	Topic    string `json:"topic,omitempty"`
	Source   string `json:"source,omitempty"`
	Status   string `json:"status"`
	Error    string `json:"error,omitempty"`
}

// Result holds the full parse outcome.
type Result struct {
	Rows        []Row `json:"rows"`
	TotalRows   int   `json:"total_rows"`
	ValidRows   int   `json:"valid_rows"`
	InvalidRows int   `json:"invalid_rows"`
}

// requiredColumnsWithDeck is the standard set when the CSV itself carries a deck column.
var requiredColumnsWithDeck = []string{"type", "question", "answer"}

// requiredColumnsNoDeck is used in single-deck mode (no deck column).
var requiredColumnsNoDeck = []string{"type", "question", "answer"}

// Parse reads a UTF-8 CSV from r and validates every row according to opts.
// A hard error is returned only for structural problems (missing required header
// columns, too many rows). Per-row validation errors are stored in Row.Error
// and do not abort the parse.
func Parse(r io.Reader, opts ParseOptions) (*Result, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	colIndex := buildColumnIndex(header)

	_, hasDeckCol := colIndex["deck"]
	if !hasDeckCol {
		if opts.DefaultDeck == "" {
			return nil, fmt.Errorf("missing required column: deck (or supply a deckId for single-deck mode)")
		}
	}

	for _, col := range requiredColumnsWithDeck {
		if _, ok := colIndex[col]; !ok {
			return nil, fmt.Errorf("missing required column: %s", col)
		}
	}

	result := &Result{}

	for lineNum := 2; ; lineNum++ {
		record, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if result.TotalRows >= MaxRows {
			return nil, fmt.Errorf("CSV exceeds maximum of %d data rows", MaxRows)
		}
		if readErr != nil {
			result.Rows = append(result.Rows, Row{
				Line:   lineNum,
				Status: "error",
				Error:  fmt.Sprintf("parse error: %v", readErr),
			})
			result.InvalidRows++
			result.TotalRows++
			continue
		}

		row := validateRow(record, colIndex, lineNum, opts)
		if row.Status == "ok" {
			result.ValidRows++
		} else {
			result.InvalidRows++
		}
		result.Rows = append(result.Rows, row)
		result.TotalRows++
	}

	return result, nil
}

func buildColumnIndex(header []string) map[string]int {
	idx := make(map[string]int, len(header))
	for i, h := range header {
		if i == 0 {
			h = strings.TrimPrefix(h, "\xef\xbb\xbf") // strip UTF-8 BOM
		}
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	return idx
}

func colVal(record []string, index map[string]int, name string) string {
	i, ok := index[name]
	if !ok || i >= len(record) {
		return ""
	}
	return sanitize(record[i])
}

func sanitize(s string) string {
	s = strings.TrimSpace(s)
	// Strip null bytes
	s = strings.Map(func(r rune) rune {
		if r == '\x00' {
			return -1
		}
		return r
	}, s)
	return s
}

// normalizeSpaces collapses runs of whitespace to a single space and trims.
// Applied to question and deck name to prevent near-duplicate entries.
func normalizeSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func validateRow(record []string, colIndex map[string]int, line int, opts ParseOptions) Row {
	_, hasDeckCol := colIndex["deck"]

	var deckVal string
	if hasDeckCol {
		deckVal = normalizeSpaces(colVal(record, colIndex, "deck"))
	} else {
		deckVal = opts.DefaultDeck
	}

	row := Row{
		Line:     line,
		Deck:     deckVal,
		Type:     sanitize(colVal(record, colIndex, "type")),
		Question: normalizeSpaces(colVal(record, colIndex, "question")),
		Answer:   sanitize(colVal(record, colIndex, "answer")),
		Topic:    sanitize(colVal(record, colIndex, "topic")),
		Source:   sanitize(colVal(record, colIndex, "source")),
	}

	var errs []string

	if row.Deck == "" {
		errs = append(errs, "deck is required")
	} else if utf8.RuneCountInString(row.Deck) > model.MaxDeckNameLen {
		errs = append(errs, fmt.Sprintf("deck name exceeds %d chars", model.MaxDeckNameLen))
	}

	if row.Question == "" {
		errs = append(errs, "question is required")
	} else if utf8.RuneCountInString(row.Question) > model.MaxQuestionLen {
		errs = append(errs, fmt.Sprintf("question exceeds %d chars", model.MaxQuestionLen))
	}

	if row.Answer == "" {
		errs = append(errs, "answer is required")
	} else if utf8.RuneCountInString(row.Answer) > model.MaxAnswerLen {
		errs = append(errs, fmt.Sprintf("answer exceeds %d chars", model.MaxAnswerLen))
	}

	ct := model.CardType(row.Type)
	if !ct.Valid() {
		errs = append(errs, fmt.Sprintf("invalid type %q; must be conceito, processo, aplicacao, or comparacao", row.Type))
	}

	if row.Topic != "" && utf8.RuneCountInString(row.Topic) > model.MaxTopicLen {
		errs = append(errs, fmt.Sprintf("topic exceeds %d chars", model.MaxTopicLen))
	}
	if row.Source != "" && utf8.RuneCountInString(row.Source) > model.MaxSourceLen {
		errs = append(errs, fmt.Sprintf("source exceeds %d chars", model.MaxSourceLen))
	}

	if len(errs) > 0 {
		row.Status = "error"
		row.Error = strings.Join(errs, "; ")
	} else {
		row.Status = "ok"
	}

	return row
}
