-- 008_perf_indexes.down.sql
DROP INDEX IF EXISTS idx_reviews_user_updated_at;
DROP INDEX IF EXISTS idx_cards_deck_topic;
DROP INDEX IF EXISTS idx_user_roles_user_id;
