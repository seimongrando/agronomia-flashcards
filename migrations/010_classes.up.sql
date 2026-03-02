-- Migration 010: Turmas (classes) + student private decks

-- Student-owned private decks are invisible to other users.
ALTER TABLE decks ADD COLUMN is_private BOOLEAN NOT NULL DEFAULT false;

-- Classes (turmas): created by professor/admin, joined by students via invite code.
CREATE TABLE classes (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(120) NOT NULL,
    description VARCHAR(500),
    invite_code VARCHAR(12)  NOT NULL UNIQUE,
    created_by  UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    is_active   BOOLEAN      NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- Students enrolled in a class (many-to-many).
CREATE TABLE class_members (
    class_id  UUID        NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    user_id   UUID        NOT NULL REFERENCES users(id)   ON DELETE CASCADE,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (class_id, user_id)
);

-- Decks assigned to a class (many-to-many).
CREATE TABLE class_decks (
    class_id UUID        NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    deck_id  UUID        NOT NULL REFERENCES decks(id)   ON DELETE CASCADE,
    added_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (class_id, deck_id)
);

CREATE INDEX idx_classes_created_by    ON classes(created_by);
CREATE INDEX idx_class_members_user_id ON class_members(user_id);
CREATE INDEX idx_class_decks_deck_id   ON class_decks(deck_id);
