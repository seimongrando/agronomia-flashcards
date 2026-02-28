-- 006_deck_active.up.sql
-- Adds manual enable/disable and optional expiration date to decks.
-- Professors can deactivate a deck or set a date after which it auto-expires.
-- Students only see decks that are active and not expired.
ALTER TABLE decks
    ADD COLUMN is_active  BOOLEAN     NOT NULL DEFAULT TRUE,
    ADD COLUMN expires_at TIMESTAMPTZ;

-- Partial index to speed up filtering inactive decks (small set, high selectivity)
CREATE INDEX idx_decks_inactive ON decks (id) WHERE NOT is_active;
