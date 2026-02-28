-- 005_deck_subject.up.sql
-- Add optional subject (discipline) to decks for grouped navigation on the home page.
-- NULL means the deck has not been assigned to a subject yet.
ALTER TABLE decks ADD COLUMN subject TEXT;

CREATE INDEX idx_decks_subject ON decks (subject) WHERE subject IS NOT NULL;
