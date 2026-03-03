DROP TABLE IF EXISTS user_deck_hidden;
ALTER TABLE decks ALTER COLUMN is_active SET DEFAULT true;
