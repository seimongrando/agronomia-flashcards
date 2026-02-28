-- 007_deck_owner.down.sql
DROP INDEX IF EXISTS idx_decks_created_by;
ALTER TABLE decks DROP COLUMN IF EXISTS created_by;
