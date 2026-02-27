-- 002_upsert_constraints.down.sql

ALTER TABLE cards DROP CONSTRAINT IF EXISTS uq_cards_deck_question;

ALTER TABLE cards
    DROP CONSTRAINT IF EXISTS cards_question_check,
    DROP CONSTRAINT IF EXISTS cards_answer_check;

ALTER TABLE cards
    ADD CONSTRAINT cards_question_check CHECK (char_length(question) <= 2000),
    ADD CONSTRAINT cards_answer_check   CHECK (char_length(answer)   <= 5000);

ALTER TABLE uploads
    ALTER COLUMN deck_id SET NOT NULL;

ALTER TABLE uploads
    DROP COLUMN IF EXISTS decks_created;
