-- Decks created by professors now start as drafts (is_active = false).
-- Professors must explicitly publish a deck before students can see it.
ALTER TABLE decks ALTER COLUMN is_active SET DEFAULT false;

-- Students can hide general decks (not private, not class-assigned) from their home page.
CREATE TABLE user_deck_hidden (
    user_id   UUID        NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    deck_id   UUID        NOT NULL REFERENCES decks(id)  ON DELETE CASCADE,
    hidden_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, deck_id)
);
CREATE INDEX idx_user_deck_hidden_user ON user_deck_hidden(user_id);
