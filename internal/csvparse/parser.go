package csvparse

import (
	"bytes"
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
	// If the CSV has no "deck" column and DefaultDeck is empty, Parse returns an error.
	DefaultDeck string

	// ForceDeck, when true, overrides the "deck" column in every row with
	// DefaultDeck. Use this when the caller has explicitly selected a deck and
	// all cards must be imported into that deck regardless of what the CSV says.
	ForceDeck bool
}

// Row represents one data row after parsing and validation.
type Row struct {
	Line     int    `json:"line"`
	Deck     string `json:"deck"`
	Subject  string `json:"subject,omitempty"` // optional discipline/subject for the deck
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

// detectSeparator inspects the first non-empty line of the CSV and returns the
// most likely field separator. Supported candidates (checked in priority order):
//
//   - Tab  '\t' — TSV files exported by some tools
//   - Semicolon ';' — common in Excel installations with European/Brazilian locale
//   - Comma ',' — RFC 4180 default
//
// Whichever candidate appears most in the first line wins. On a tie the order
// above is used as tiebreaker (tab > semicolon > comma).
func detectSeparator(firstLine string) rune {
	counts := map[rune]int{
		'\t': strings.Count(firstLine, "\t"),
		';':  strings.Count(firstLine, ";"),
		',':  strings.Count(firstLine, ","),
	}
	best := ','
	bestN := counts[',']
	for _, c := range []rune{'\t', ';'} {
		if counts[c] > bestN {
			best = c
			bestN = counts[c]
		}
	}
	return best
}

// Parse reads a UTF-8 CSV from r and validates every row according to opts.
//
// The separator is auto-detected from the header line: comma, semicolon, and
// tab are all supported. Text fields containing the separator must be enclosed
// in double-quote characters (RFC 4180). LazyQuotes is enabled so slightly
// malformed quoting does not abort the import.
//
// A hard error is returned only for structural problems (missing required header
// columns, too many rows). Per-row validation errors are stored in Row.Error
// and do not abort the parse.
func Parse(r io.Reader, opts ParseOptions) (*Result, error) {
	// Read entire body so we can (a) detect the separator from the first line
	// without consuming the reader, and (b) enforce the size limit before the
	// csv.Reader allocates anything.
	raw, err := io.ReadAll(io.LimitReader(r, MaxFileSize+1))
	if err != nil {
		return nil, fmt.Errorf("erro ao ler arquivo: %w", err)
	}
	if len(raw) > MaxFileSize {
		return nil, fmt.Errorf("arquivo excede o limite de %d MB", MaxFileSize>>20)
	}

	// Strip UTF-8 BOM that Excel/Windows sometimes prepends.
	raw = bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))

	// Detect separator from the first non-empty line.
	firstLine := ""
	for _, line := range strings.SplitN(string(raw), "\n", 3) {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			firstLine = trimmed
			break
		}
	}
	sep := detectSeparator(firstLine)

	reader := csv.NewReader(bytes.NewReader(raw))
	reader.Comma = sep
	reader.TrimLeadingSpace = true
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("falha ao ler cabeçalho do CSV: %w", err)
	}

	colIndex := buildColumnIndex(header)

	_, hasDeckCol := colIndex["deck"]
	if !hasDeckCol {
		if opts.DefaultDeck == "" {
			return nil, fmt.Errorf("coluna obrigatória ausente: deck (ou informe um deckId para modo de deck único)")
		}
	}

	for _, col := range requiredColumnsWithDeck {
		if _, ok := colIndex[col]; !ok {
			return nil, fmt.Errorf("coluna obrigatória ausente: %s", col)
		}
	}

	result := &Result{}

	for lineNum := 2; ; lineNum++ {
		record, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if result.TotalRows >= MaxRows {
			return nil, fmt.Errorf("CSV excede o máximo de %d linhas de dados", MaxRows)
		}
		if readErr != nil {
			result.Rows = append(result.Rows, Row{
				Line:   lineNum,
				Status: "error",
				Error:  fmt.Sprintf("erro de leitura na linha: %v", readErr),
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
	switch {
	case opts.ForceDeck:
		// Caller has explicitly selected a target deck; ignore the CSV column.
		deckVal = opts.DefaultDeck
	case hasDeckCol:
		deckVal = normalizeSpaces(colVal(record, colIndex, "deck"))
	default:
		deckVal = opts.DefaultDeck
	}

	row := Row{
		Line:     line,
		Deck:     deckVal,
		Subject:  sanitize(colVal(record, colIndex, "subject")), // optional
		Type:     sanitize(colVal(record, colIndex, "type")),
		Question: normalizeSpaces(colVal(record, colIndex, "question")),
		Answer:   sanitize(colVal(record, colIndex, "answer")),
		Topic:    sanitize(colVal(record, colIndex, "topic")),
		Source:   sanitize(colVal(record, colIndex, "source")),
	}

	var errs []string

	if row.Deck == "" {
		errs = append(errs, "deck é obrigatório")
	} else if utf8.RuneCountInString(row.Deck) > model.MaxDeckNameLen {
		errs = append(errs, fmt.Sprintf("nome do deck excede %d caracteres", model.MaxDeckNameLen))
	}

	if row.Question == "" {
		errs = append(errs, "pergunta é obrigatória")
	} else if utf8.RuneCountInString(row.Question) > model.MaxQuestionLen {
		errs = append(errs, fmt.Sprintf("pergunta excede %d caracteres", model.MaxQuestionLen))
	}

	if row.Answer == "" {
		errs = append(errs, "resposta é obrigatória")
	} else if utf8.RuneCountInString(row.Answer) > model.MaxAnswerLen {
		errs = append(errs, fmt.Sprintf("resposta excede %d caracteres", model.MaxAnswerLen))
	}

	ct := model.CardType(row.Type)
	if !ct.Valid() {
		errs = append(errs, fmt.Sprintf("tipo inválido %q; deve ser: conceito, processo, aplicacao ou comparacao", row.Type))
	}

	if row.Topic != "" && utf8.RuneCountInString(row.Topic) > model.MaxTopicLen {
		errs = append(errs, fmt.Sprintf("tópico excede %d caracteres", model.MaxTopicLen))
	}
	if row.Source != "" && utf8.RuneCountInString(row.Source) > model.MaxSourceLen {
		errs = append(errs, fmt.Sprintf("fonte excede %d caracteres", model.MaxSourceLen))
	}

	if len(errs) > 0 {
		row.Status = "error"
		row.Error = strings.Join(errs, "; ")
	} else {
		row.Status = "ok"
	}

	return row
}
