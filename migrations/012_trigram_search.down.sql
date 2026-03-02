DROP INDEX IF EXISTS idx_cards_question_trgm;
DROP INDEX IF EXISTS idx_cards_topic_trgm;
-- Note: pg_trgm extension is intentionally NOT dropped here as other
-- indexes or queries in the schema may depend on it.
