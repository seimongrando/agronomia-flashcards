-- Migration 009: indexes to speed up the deck-list and subject-grouping queries.
--
-- The home-page query does:
--   LEFT JOIN reviews rv ON rv.card_id = c.id AND rv.user_id = $1
-- iterating over cards per deck in a nested-loop join.
-- The existing UNIQUE index is on (user_id, card_id); adding the reverse
-- (card_id, user_id) lets PostgreSQL start from the card side efficiently.
CREATE INDEX IF NOT EXISTS idx_reviews_card_user
    ON reviews (card_id, user_id);

-- Subject grouping / filtering on decks
CREATE INDEX IF NOT EXISTS idx_decks_subject
    ON decks (subject)
    WHERE subject IS NOT NULL;
