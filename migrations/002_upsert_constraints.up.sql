-- 002_upsert_constraints.up.sql

-- ── 1. Deduplicate existing cards before enforcing uniqueness ──────────────────
-- Keep the most recently updated row when multiple cards share (deck_id, question).
DELETE FROM cards
WHERE id NOT IN (
    SELECT DISTINCT ON (deck_id, question) id
    FROM cards
    ORDER BY deck_id, question, updated_at DESC
);

-- ── 2. Enforce UNIQUE(deck_id, question) ──────────────────────────────────────
ALTER TABLE cards
    ADD CONSTRAINT uq_cards_deck_question UNIQUE (deck_id, question);

-- ── 3. Tighten length CHECK constraints to match app-level validation ─────────
ALTER TABLE cards
    DROP CONSTRAINT IF EXISTS cards_question_check,
    DROP CONSTRAINT IF EXISTS cards_answer_check;

ALTER TABLE cards
    ADD CONSTRAINT cards_question_check CHECK (char_length(question) <= 500),
    ADD CONSTRAINT cards_answer_check   CHECK (char_length(answer)   <= 2000);

-- ── 4. uploads: make deck_id nullable (multi-deck imports span many decks) ─────
ALTER TABLE uploads
    ALTER COLUMN deck_id DROP NOT NULL;

-- ── 5. uploads: track how many new decks were created during an import ─────────
ALTER TABLE uploads
    ADD COLUMN decks_created INT NOT NULL DEFAULT 0;
