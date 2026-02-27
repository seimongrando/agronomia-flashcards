// Package pagination provides cursor-based pagination helpers for HTTP list
// endpoints.
//
// Cursors are opaque, URL-safe base64-encoded JSON tokens that encode the
// ordering fields of the last item on the current page. They are deterministic
// and stable: re-encoding the same fields always produces the same token.
//
// Two cursor families are supported:
//   - TimestampID  — encodes (timestamptz, uuid), used for DESC-ordered lists.
//   - NameID       — encodes (name string, uuid), used for ASC name-ordered lists.
package pagination

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

const (
	DefaultLimit = 50
	MaxLimit     = 100
)

// Page is the standard envelope for all paginated list responses.
// NextCursor is nil when no further pages exist.
type Page[T any] struct {
	Items      []T     `json:"items"`
	NextCursor *string `json:"next_cursor"`
}

// ParseLimit reads the "limit" query parameter, validates it, and returns the
// effective limit. An error is returned (and should result in HTTP 400) when
// the value is present but outside [1, maxVal].
func ParseLimit(r *http.Request, defaultVal, maxVal int) (int, error) {
	s := r.URL.Query().Get("limit")
	if s == "" {
		return defaultVal, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > maxVal {
		return 0, fmt.Errorf("limit must be an integer between 1 and %d", maxVal)
	}
	return n, nil
}

// ─── Timestamp + UUID cursor ──────────────────────────────────────────────────

type tsIDPayload struct {
	TS string `json:"ts"`
	ID string `json:"id"`
}

// EncodeTimestampIDCursor encodes a (timestamptz, uuid) pair into an opaque
// cursor string. Used for endpoints ordered by (column DESC, id DESC).
func EncodeTimestampIDCursor(ts time.Time, id string) string {
	b, _ := json.Marshal(tsIDPayload{
		TS: ts.UTC().Format(time.RFC3339Nano),
		ID: id,
	})
	return base64.URLEncoding.EncodeToString(b)
}

// DecodeTimestampIDCursor decodes a cursor produced by EncodeTimestampIDCursor.
// Returns a descriptive error (suitable for HTTP 400) on any malformed input.
func DecodeTimestampIDCursor(s string) (time.Time, string, error) {
	raw, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("invalid cursor")
	}
	var p tsIDPayload
	if err := json.Unmarshal(raw, &p); err != nil || p.TS == "" || p.ID == "" {
		return time.Time{}, "", fmt.Errorf("invalid cursor")
	}
	ts, err := time.Parse(time.RFC3339Nano, p.TS)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("invalid cursor")
	}
	return ts, p.ID, nil
}

// ─── Name + UUID cursor ───────────────────────────────────────────────────────

type nameIDPayload struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

// EncodeNameIDCursor encodes a (name, uuid) pair into an opaque cursor string.
// Used for endpoints ordered by (name ASC, id ASC).
func EncodeNameIDCursor(name, id string) string {
	b, _ := json.Marshal(nameIDPayload{Name: name, ID: id})
	return base64.URLEncoding.EncodeToString(b)
}

// DecodeNameIDCursor decodes a cursor produced by EncodeNameIDCursor.
func DecodeNameIDCursor(s string) (string, string, error) {
	raw, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return "", "", fmt.Errorf("invalid cursor")
	}
	var p nameIDPayload
	if err := json.Unmarshal(raw, &p); err != nil || p.ID == "" {
		return "", "", fmt.Errorf("invalid cursor")
	}
	return p.Name, p.ID, nil
}
