package pagination_test

import (
	"net/http/httptest"
	"testing"
	"time"

	"webapp/internal/pagination"
)

// ── ParseLimit ────────────────────────────────────────────────────────────────

func TestParseLimit_Default(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	got, err := pagination.ParseLimit(r, 50, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 50 {
		t.Errorf("got %d, want 50", got)
	}
}

func TestParseLimit_Valid(t *testing.T) {
	r := httptest.NewRequest("GET", "/?limit=10", nil)
	got, err := pagination.ParseLimit(r, 50, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 10 {
		t.Errorf("got %d, want 10", got)
	}
}

func TestParseLimit_Max(t *testing.T) {
	r := httptest.NewRequest("GET", "/?limit=100", nil)
	got, err := pagination.ParseLimit(r, 50, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 100 {
		t.Errorf("got %d, want 100", got)
	}
}

func TestParseLimit_TooLarge(t *testing.T) {
	r := httptest.NewRequest("GET", "/?limit=101", nil)
	_, err := pagination.ParseLimit(r, 50, 100)
	if err == nil {
		t.Fatal("expected error for limit > max")
	}
}

func TestParseLimit_Zero(t *testing.T) {
	r := httptest.NewRequest("GET", "/?limit=0", nil)
	_, err := pagination.ParseLimit(r, 50, 100)
	if err == nil {
		t.Fatal("expected error for limit=0")
	}
}

func TestParseLimit_Negative(t *testing.T) {
	r := httptest.NewRequest("GET", "/?limit=-5", nil)
	_, err := pagination.ParseLimit(r, 50, 100)
	if err == nil {
		t.Fatal("expected error for negative limit")
	}
}

func TestParseLimit_NonNumeric(t *testing.T) {
	r := httptest.NewRequest("GET", "/?limit=abc", nil)
	_, err := pagination.ParseLimit(r, 50, 100)
	if err == nil {
		t.Fatal("expected error for non-numeric limit")
	}
}

// ── TimestampID cursor ────────────────────────────────────────────────────────

func TestTimestampIDCursor_RoundTrip(t *testing.T) {
	ts := time.Date(2025, 6, 1, 12, 30, 45, 123456789, time.UTC)
	id := "550e8400-e29b-41d4-a716-446655440000"

	encoded := pagination.EncodeTimestampIDCursor(ts, id)
	if encoded == "" {
		t.Fatal("encoded cursor is empty")
	}

	gotTS, gotID, err := pagination.DecodeTimestampIDCursor(encoded)
	if err != nil {
		t.Fatalf("unexpected error decoding: %v", err)
	}
	if !gotTS.Equal(ts) {
		t.Errorf("ts: got %v, want %v", gotTS, ts)
	}
	if gotID != id {
		t.Errorf("id: got %q, want %q", gotID, id)
	}
}

func TestTimestampIDCursor_Deterministic(t *testing.T) {
	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	id := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"

	c1 := pagination.EncodeTimestampIDCursor(ts, id)
	c2 := pagination.EncodeTimestampIDCursor(ts, id)
	if c1 != c2 {
		t.Errorf("encoding is not deterministic: %q != %q", c1, c2)
	}
}

func TestTimestampIDCursor_InvalidBase64(t *testing.T) {
	_, _, err := pagination.DecodeTimestampIDCursor("not!valid!base64")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestTimestampIDCursor_InvalidJSON(t *testing.T) {
	import64 := "aW52YWxpZA==" // base64("invalid")
	_, _, err := pagination.DecodeTimestampIDCursor(import64)
	if err == nil {
		t.Fatal("expected error for invalid JSON payload")
	}
}

func TestTimestampIDCursor_Empty(t *testing.T) {
	_, _, err := pagination.DecodeTimestampIDCursor("")
	if err == nil {
		t.Fatal("expected error for empty cursor")
	}
}

// ── NameID cursor ─────────────────────────────────────────────────────────────

func TestNameIDCursor_RoundTrip(t *testing.T) {
	name := "Biologia Celular"
	id := "123e4567-e89b-12d3-a456-426614174000"

	encoded := pagination.EncodeNameIDCursor(name, id)
	if encoded == "" {
		t.Fatal("encoded cursor is empty")
	}

	gotName, gotID, err := pagination.DecodeNameIDCursor(encoded)
	if err != nil {
		t.Fatalf("unexpected error decoding: %v", err)
	}
	if gotName != name {
		t.Errorf("name: got %q, want %q", gotName, name)
	}
	if gotID != id {
		t.Errorf("id: got %q, want %q", gotID, id)
	}
}

func TestNameIDCursor_EmptyName(t *testing.T) {
	// Empty name is valid (first position in alphabetical order).
	id := "123e4567-e89b-12d3-a456-426614174000"
	encoded := pagination.EncodeNameIDCursor("", id)
	gotName, gotID, err := pagination.DecodeNameIDCursor(encoded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotName != "" {
		t.Errorf("name: got %q, want empty", gotName)
	}
	if gotID != id {
		t.Errorf("id: got %q, want %q", gotID, id)
	}
}

func TestNameIDCursor_Deterministic(t *testing.T) {
	c1 := pagination.EncodeNameIDCursor("Math", "aaa")
	c2 := pagination.EncodeNameIDCursor("Math", "aaa")
	if c1 != c2 {
		t.Errorf("encoding is not deterministic")
	}
}

func TestNameIDCursor_Invalid(t *testing.T) {
	_, _, err := pagination.DecodeNameIDCursor("!!garbage!!")
	if err == nil {
		t.Fatal("expected error for invalid cursor")
	}
}
