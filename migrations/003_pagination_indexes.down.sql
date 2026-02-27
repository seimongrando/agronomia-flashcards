-- 003_pagination_indexes.down.sql

DROP INDEX IF EXISTS idx_cards_deck_updated_id;
DROP INDEX IF EXISTS idx_users_created_id;
DROP INDEX IF EXISTS idx_decks_name_id;
DROP INDEX IF EXISTS idx_uploads_user_created;
DROP INDEX IF EXISTS idx_reviews_user_updated;
