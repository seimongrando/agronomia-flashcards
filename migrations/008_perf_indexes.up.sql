-- 008_perf_indexes.up.sql
-- Performance indexes identified during database audit.

-- ── reviews (user_id, updated_at DESC) ───────────────────────────────────────
-- The existing idx_reviews_user_updated is a PARTIAL index (WHERE last_result = 0)
-- and can only serve wrong-card queries.
-- The streak calculation (GlobalProgress), the Stats subqueries, and the
-- GlobalProgress per-deck breakdown all filter by (user_id, updated_at) without
-- a last_result restriction — they currently fall back to a sequential scan of
-- the user's review rows.
CREATE INDEX IF NOT EXISTS idx_reviews_user_updated_at
    ON reviews (user_id, updated_at DESC);

-- ── cards (deck_id, topic) ───────────────────────────────────────────────────
-- DeckTopics runs: SELECT DISTINCT topic FROM cards WHERE deck_id = $1 AND topic IS NOT NULL
-- Without this index every row for the deck must be heap-fetched to read topic.
-- With a covering composite index the query becomes a pure index scan.
CREATE INDEX IF NOT EXISTS idx_cards_deck_topic
    ON cards (deck_id, topic)
    WHERE topic IS NOT NULL;

-- ── user_roles (user_id) ─────────────────────────────────────────────────────
-- RolesByUserID is called on every authenticated request (Me handler, JWT validation).
-- The UNIQUE constraint on (user_id, role) creates a btree index, so lookups by
-- user_id are already index-supported. The explicit index below makes the scan
-- intent visible and guards against future constraint changes.
-- (No-op if the UNIQUE index already covers this efficiently — PG deduplicates.)
CREATE INDEX IF NOT EXISTS idx_user_roles_user_id
    ON user_roles (user_id);
