package csvparse

import (
	"strings"
	"testing"

	"webapp/internal/model"
)

// defaultOpts returns a ParseOptions with no special settings (multi-deck mode).
func defaultOpts() ParseOptions { return ParseOptions{} }

// ── Standard multi-deck CSV ───────────────────────────────────────────────────

func TestParse_ValidCSV(t *testing.T) {
	csv := "deck,type,question,answer,topic,source\n" +
		"Bio,conceito,O que é DNA?,Ácido desoxirribonucleico,Genética,Livro p.10\n" +
		"Bio,processo,Fases da mitose?,Prófase Metáfase Anáfase Telófase,,\n"

	result, err := Parse(strings.NewReader(csv), defaultOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalRows != 2 {
		t.Errorf("TotalRows = %d; want 2", result.TotalRows)
	}
	if result.ValidRows != 2 {
		t.Errorf("ValidRows = %d; want 2", result.ValidRows)
	}
	if result.InvalidRows != 0 {
		t.Errorf("InvalidRows = %d; want 0", result.InvalidRows)
	}

	row := result.Rows[0]
	if row.Deck != "Bio" {
		t.Errorf("Deck = %q; want Bio", row.Deck)
	}
	if row.Type != "conceito" {
		t.Errorf("Type = %q; want conceito", row.Type)
	}
	if row.Topic != "Genética" {
		t.Errorf("Topic = %q; want Genética", row.Topic)
	}
	if row.Source != "Livro p.10" {
		t.Errorf("Source = %q; want Livro p.10", row.Source)
	}
	if row.Status != "ok" {
		t.Errorf("Status = %q; want ok", row.Status)
	}

	row2 := result.Rows[1]
	if row2.Topic != "" {
		t.Errorf("Topic should be empty; got %q", row2.Topic)
	}
	if row2.Source != "" {
		t.Errorf("Source should be empty; got %q", row2.Source)
	}
}

// ── BOM handling ──────────────────────────────────────────────────────────────

func TestParse_BOMHandling(t *testing.T) {
	csv := "\xef\xbb\xbfdeck,type,question,answer\n" +
		"Math,conceito,1+1?,2\n"

	result, err := Parse(strings.NewReader(csv), defaultOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ValidRows != 1 {
		t.Errorf("ValidRows = %d; want 1", result.ValidRows)
	}
	if result.Rows[0].Deck != "Math" {
		t.Errorf("Deck = %q; want Math", result.Rows[0].Deck)
	}
}

// ── Missing required columns ──────────────────────────────────────────────────

func TestParse_MissingRequiredColumn_Answer(t *testing.T) {
	csv := "deck,type,question\nBio,conceito,What?\n"

	_, err := Parse(strings.NewReader(csv), defaultOpts())
	if err == nil {
		t.Fatal("expected error for missing 'answer' column")
	}
	if !strings.Contains(err.Error(), "missing required column: answer") {
		t.Errorf("error = %q; want 'missing required column: answer'", err.Error())
	}
}

func TestParse_MissingDeckColumn_NoDeckID(t *testing.T) {
	// No deck column and no default deck → hard error
	csv := "type,question,answer\nconceito,Q?,A\n"
	_, err := Parse(strings.NewReader(csv), defaultOpts())
	if err == nil {
		t.Fatal("expected error when deck column missing and no DefaultDeck")
	}
	if !strings.Contains(err.Error(), "missing required column: deck") {
		t.Errorf("error = %q; want 'missing required column: deck'", err.Error())
	}
}

func TestParse_EmptyHeader(t *testing.T) {
	_, err := Parse(strings.NewReader(""), defaultOpts())
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

// ── Per-row validation errors ─────────────────────────────────────────────────

func TestParse_InvalidType(t *testing.T) {
	csv := "deck,type,question,answer\nBio,invalid_type,Q?,A\n"

	result, err := Parse(strings.NewReader(csv), defaultOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.InvalidRows != 1 {
		t.Errorf("InvalidRows = %d; want 1", result.InvalidRows)
	}
	if !strings.Contains(result.Rows[0].Error, "invalid type") {
		t.Errorf("Error = %q; want contains 'invalid type'", result.Rows[0].Error)
	}
}

func TestParse_MissingRequiredFields(t *testing.T) {
	csv := "deck,type,question,answer\n" +
		",conceito,Q,A\n" + // missing deck
		"Bio,conceito,,A\n" + // missing question
		"Bio,conceito,Q,\n" // missing answer

	result, err := Parse(strings.NewReader(csv), defaultOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.InvalidRows != 3 {
		t.Errorf("InvalidRows = %d; want 3", result.InvalidRows)
	}
	if !strings.Contains(result.Rows[0].Error, "deck is required") {
		t.Errorf("Row 1 error = %q; want contains 'deck is required'", result.Rows[0].Error)
	}
	if !strings.Contains(result.Rows[1].Error, "question is required") {
		t.Errorf("Row 2 error = %q; want contains 'question is required'", result.Rows[1].Error)
	}
	if !strings.Contains(result.Rows[2].Error, "answer is required") {
		t.Errorf("Row 3 error = %q; want contains 'answer is required'", result.Rows[2].Error)
	}
}

func TestParse_MultipleErrorsPerRow(t *testing.T) {
	csv := "deck,type,question,answer\n,bad_type,,\n"

	result, err := Parse(strings.NewReader(csv), defaultOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	row := result.Rows[0]
	if row.Status != "error" {
		t.Fatalf("Status = %q; want error", row.Status)
	}
	for _, want := range []string{"deck is required", "question is required", "answer is required", "invalid type"} {
		if !strings.Contains(row.Error, want) {
			t.Errorf("Error %q missing %q", row.Error, want)
		}
	}
}

// ── Length limits ─────────────────────────────────────────────────────────────

func TestParse_QuestionTooLong(t *testing.T) {
	longQ := strings.Repeat("x", model.MaxQuestionLen+1)
	csv := "deck,type,question,answer\nBio,conceito," + longQ + ",A\n"

	result, err := Parse(strings.NewReader(csv), defaultOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.InvalidRows != 1 {
		t.Errorf("InvalidRows = %d; want 1", result.InvalidRows)
	}
	if !strings.Contains(result.Rows[0].Error, "question exceeds") {
		t.Errorf("Error = %q; want contains 'question exceeds'", result.Rows[0].Error)
	}
}

func TestParse_AnswerTooLong(t *testing.T) {
	longA := strings.Repeat("y", model.MaxAnswerLen+1)
	csv := "deck,type,question,answer\nBio,conceito,Q?," + longA + "\n"

	result, err := Parse(strings.NewReader(csv), defaultOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.InvalidRows != 1 {
		t.Errorf("InvalidRows = %d; want 1", result.InvalidRows)
	}
	if !strings.Contains(result.Rows[0].Error, "answer exceeds") {
		t.Errorf("Error = %q; want contains 'answer exceeds'", result.Rows[0].Error)
	}
}

func TestParse_TopicTooLong(t *testing.T) {
	longTopic := strings.Repeat("t", model.MaxTopicLen+1)
	csv := "deck,type,question,answer,topic\nBio,conceito,Q,A," + longTopic + "\n"

	result, err := Parse(strings.NewReader(csv), defaultOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.InvalidRows != 1 {
		t.Errorf("InvalidRows = %d; want 1", result.InvalidRows)
	}
	if !strings.Contains(result.Rows[0].Error, "topic exceeds") {
		t.Errorf("Error = %q; want contains 'topic exceeds'", result.Rows[0].Error)
	}
}

// ── Row / file limits ─────────────────────────────────────────────────────────

func TestParse_ExceedsMaxRows(t *testing.T) {
	var b strings.Builder
	b.WriteString("deck,type,question,answer\n")
	for i := 0; i <= MaxRows; i++ {
		b.WriteString("Bio,conceito,Q,A\n")
	}

	_, err := Parse(strings.NewReader(b.String()), defaultOpts())
	if err == nil {
		t.Fatal("expected error for exceeding MaxRows")
	}
	if !strings.Contains(err.Error(), "exceeds maximum") {
		t.Errorf("error = %q; want contains 'exceeds maximum'", err.Error())
	}
}

// ── Extra columns, case insensitive headers ───────────────────────────────────

func TestParse_ExtraColumnsIgnored(t *testing.T) {
	csv := "deck,type,question,answer,topic,source,extra\nBio,conceito,Q?,A,T,S,ignored\n"

	result, err := Parse(strings.NewReader(csv), defaultOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ValidRows != 1 {
		t.Errorf("ValidRows = %d; want 1", result.ValidRows)
	}
}

func TestParse_CaseInsensitiveHeaders(t *testing.T) {
	csv := "Deck,TYPE,Question,ANSWER\nBio,conceito,Q?,A\n"

	result, err := Parse(strings.NewReader(csv), defaultOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ValidRows != 1 {
		t.Errorf("ValidRows = %d; want 1", result.ValidRows)
	}
}

// ── Whitespace & sanitisation ─────────────────────────────────────────────────

func TestParse_TrimsWhitespace(t *testing.T) {
	csv := "deck,type,question,answer\n  Bio  , conceito , Q? , A \n"

	result, err := Parse(strings.NewReader(csv), defaultOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	row := result.Rows[0]
	if row.Deck != "Bio" {
		t.Errorf("Deck = %q; want 'Bio'", row.Deck)
	}
	if row.Type != "conceito" {
		t.Errorf("Type = %q; want 'conceito'", row.Type)
	}
}

func TestParse_CollapseSpacesInQuestion(t *testing.T) {
	// Multiple internal spaces should collapse to one for dedup normalisation.
	csv := "deck,type,question,answer\nBio,conceito,O  que   é   DNA?,Ácido\n"

	result, err := Parse(strings.NewReader(csv), defaultOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Rows[0].Question != "O que é DNA?" {
		t.Errorf("Question = %q; want 'O que é DNA?'", result.Rows[0].Question)
	}
}

func TestParse_CollapseSpacesInDeck(t *testing.T) {
	csv := "deck,type,question,answer\nBio  Med,conceito,Q?,A\n"

	result, err := Parse(strings.NewReader(csv), defaultOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Rows[0].Deck != "Bio Med" {
		t.Errorf("Deck = %q; want 'Bio Med'", result.Rows[0].Deck)
	}
}

func TestParse_NullBytesStripped(t *testing.T) {
	csv := "deck,type,question,answer\nBio,conceito,Q\x00?,A\n"

	result, err := Parse(strings.NewReader(csv), defaultOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Rows[0].Question != "Q?" {
		t.Errorf("Question = %q; want 'Q?'", result.Rows[0].Question)
	}
}

// ── All valid card types ──────────────────────────────────────────────────────

func TestParse_AllCardTypes(t *testing.T) {
	for _, ct := range []string{"conceito", "processo", "aplicacao", "comparacao"} {
		csv := "deck,type,question,answer\nD," + ct + ",Q,A\n"
		result, err := Parse(strings.NewReader(csv), defaultOpts())
		if err != nil {
			t.Fatalf("type %s: unexpected error: %v", ct, err)
		}
		if result.ValidRows != 1 {
			t.Errorf("type %s: ValidRows = %d; want 1", ct, result.ValidRows)
		}
	}
}

// ── Quoted fields and embedded commas ────────────────────────────────────────

func TestParse_QuotedFields(t *testing.T) {
	csv := "deck,type,question,answer\n" +
		`"Bio","conceito","What is ""DNA""?","It is a molecule"` + "\n"

	result, err := Parse(strings.NewReader(csv), defaultOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ValidRows != 1 {
		t.Fatalf("ValidRows = %d; want 1", result.ValidRows)
	}
	if result.Rows[0].Question != `What is "DNA"?` {
		t.Errorf("Question = %q; want 'What is \"DNA\"?'", result.Rows[0].Question)
	}
}

func TestParse_CommaInsideField(t *testing.T) {
	// Commas inside quoted fields must not split the column.
	csv := "deck,type,question,answer\n" +
		`Bio,conceito,"A, B, or C?","A, B, and C"` + "\n"

	result, err := Parse(strings.NewReader(csv), defaultOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ValidRows != 1 {
		t.Fatalf("ValidRows = %d; want 1", result.ValidRows)
	}
	if result.Rows[0].Question != "A, B, or C?" {
		t.Errorf("Question = %q; want 'A, B, or C?'", result.Rows[0].Question)
	}
	if result.Rows[0].Answer != "A, B, and C" {
		t.Errorf("Answer = %q; want 'A, B, and C'", result.Rows[0].Answer)
	}
}

// ── Empty data rows ───────────────────────────────────────────────────────────

func TestParse_NoDataRows(t *testing.T) {
	csv := "deck,type,question,answer\n"

	result, err := Parse(strings.NewReader(csv), defaultOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalRows != 0 {
		t.Errorf("TotalRows = %d; want 0", result.TotalRows)
	}
}

// ── Line counting ─────────────────────────────────────────────────────────────

func TestParse_LineCounting(t *testing.T) {
	csv := "deck,type,question,answer\nBio,conceito,Q1,A1\nBio,conceito,Q2,A2\n"

	result, err := Parse(strings.NewReader(csv), defaultOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Rows[0].Line != 2 {
		t.Errorf("Row 0 Line = %d; want 2", result.Rows[0].Line)
	}
	if result.Rows[1].Line != 3 {
		t.Errorf("Row 1 Line = %d; want 3", result.Rows[1].Line)
	}
}

// ── Single-deck mode ──────────────────────────────────────────────────────────

func TestParse_SingleDeckMode_NoDeckColumn(t *testing.T) {
	// No "deck" column; DefaultDeck is supplied via ParseOptions.
	csv := "type,question,answer\nconceito,O que é DNA?,Ácido desoxirribonucleico\n"
	opts := ParseOptions{DefaultDeck: "Biologia"}

	result, err := Parse(strings.NewReader(csv), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ValidRows != 1 {
		t.Fatalf("ValidRows = %d; want 1", result.ValidRows)
	}
	if result.Rows[0].Deck != "Biologia" {
		t.Errorf("Deck = %q; want 'Biologia'", result.Rows[0].Deck)
	}
}

func TestParse_SingleDeckMode_WithTopic(t *testing.T) {
	csv := "type,question,answer,topic,source\n" +
		"processo,Como ocorre a mitose?,Divisão celular,Citologia,Livro p.5\n"
	opts := ParseOptions{DefaultDeck: "Bio"}

	result, err := Parse(strings.NewReader(csv), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ValidRows != 1 {
		t.Fatalf("ValidRows = %d; want 1", result.ValidRows)
	}
	row := result.Rows[0]
	if row.Deck != "Bio" {
		t.Errorf("Deck = %q; want 'Bio'", row.Deck)
	}
	if row.Topic != "Citologia" {
		t.Errorf("Topic = %q; want 'Citologia'", row.Topic)
	}
	if row.Source != "Livro p.5" {
		t.Errorf("Source = %q; want 'Livro p.5'", row.Source)
	}
}

func TestParse_SingleDeckMode_DeckColumnPresent(t *testing.T) {
	// If the CSV itself has a deck column, DefaultDeck is ignored.
	csv := "deck,type,question,answer\nChemistry,conceito,H2O?,Water\n"
	opts := ParseOptions{DefaultDeck: "ShouldBeIgnored"}

	result, err := Parse(strings.NewReader(csv), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Rows[0].Deck != "Chemistry" {
		t.Errorf("Deck = %q; want 'Chemistry' (deck column takes precedence)", result.Rows[0].Deck)
	}
}

func TestParse_SingleDeckMode_MultipleRows(t *testing.T) {
	csv := "type,question,answer\n" +
		"conceito,Q1,A1\n" +
		"processo,Q2,A2\n" +
		"aplicacao,Q3,A3\n"
	opts := ParseOptions{DefaultDeck: "TestDeck"}

	result, err := Parse(strings.NewReader(csv), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ValidRows != 3 {
		t.Errorf("ValidRows = %d; want 3", result.ValidRows)
	}
	for i, row := range result.Rows {
		if row.Deck != "TestDeck" {
			t.Errorf("Row %d Deck = %q; want 'TestDeck'", i, row.Deck)
		}
	}
}
