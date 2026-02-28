-- 005_deck_subject.down.sql
DROP INDEX IF EXISTS idx_decks_subject;
ALTER TABLE decks DROP COLUMN IF EXISTS subject;
