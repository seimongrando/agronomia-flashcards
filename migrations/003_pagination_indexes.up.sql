-- 003_pagination_indexes.up.sql
-- Indexes to support cursor-based pagination and avoid expensive sequential scans.

-- ── GET /api/content/cards (professor/admin) ──────────────────────────────────
-- Ordered by (updated_at DESC, id DESC) within a deck.
-- Covers the cursor condition AND the deck filter.
CREATE INDEX IF NOT EXISTS idx_cards_deck_updated_id
    ON cards (deck_id, updated_at DESC, id DESC);

-- Note: ILIKE search on question/topic (?q=...) currently falls back to a
-- sequential scan of the deck subset. For large decks, add a GIN trigram index:
--   CREATE EXTENSION IF NOT EXISTS pg_trgm;
--   CREATE INDEX idx_cards_question_trgm ON cards USING gin (question gin_trgm_ops);
--   CREATE INDEX idx_cards_topic_trgm    ON cards USING gin (topic    gin_trgm_ops);
-- This is documented as a future improvement (requires pg_trgm extension).

-- ── GET /api/admin/users (admin) ──────────────────────────────────────────────
-- Ordered by (created_at DESC, id DESC).
CREATE INDEX IF NOT EXISTS idx_users_created_id
    ON users (created_at DESC, id DESC);

-- ── GET /api/decks (authenticated) ────────────────────────────────────────────
-- Ordered by (name ASC, id ASC).
-- decks.name already has a UNIQUE index; this covers both the filter and the
-- cursor condition without a separate index.
-- (UNIQUE constraint on name implies an index on name, but not a composite with id.)
CREATE INDEX IF NOT EXISTS idx_decks_name_id
    ON decks (name ASC, id ASC);

-- ── Uploads audit ─────────────────────────────────────────────────────────────
-- Support listing uploads by user (LGPD access requests, admin audit).
CREATE INDEX IF NOT EXISTS idx_uploads_user_created
    ON uploads (user_id, created_at DESC);

-- ── Reviews: wrong-card queries ───────────────────────────────────────────────
-- NextWrongCard queries by user + last_result + updated_at.
-- Complements the existing idx_reviews_user_due.
CREATE INDEX IF NOT EXISTS idx_reviews_user_updated
    ON reviews (user_id, updated_at DESC)
    WHERE last_result = 0;
