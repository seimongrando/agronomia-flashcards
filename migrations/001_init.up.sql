-- 001_init.up.sql — Flashcard system schema

CREATE TABLE users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    google_sub  TEXT UNIQUE NOT NULL,
    email       TEXT NOT NULL,
    name        TEXT NOT NULL,
    picture_url TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_users_email      ON users (email);
CREATE INDEX idx_users_google_sub ON users (google_sub);

CREATE TABLE user_roles (
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       TEXT        NOT NULL CHECK (role IN ('admin', 'professor', 'student')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, role)
);

CREATE TABLE decks (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT UNIQUE NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE cards (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deck_id    UUID NOT NULL REFERENCES decks(id) ON DELETE CASCADE,
    topic      TEXT,
    type       TEXT NOT NULL CHECK (type IN ('conceito', 'processo', 'aplicacao', 'comparacao')),
    question   TEXT NOT NULL CHECK (char_length(question) <= 2000),
    answer     TEXT NOT NULL CHECK (char_length(answer) <= 5000),
    source     TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_cards_deck_id ON cards (deck_id);

CREATE TABLE reviews (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    card_id     UUID        NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    next_due    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_result SMALLINT    NOT NULL DEFAULT 0,
    streak      INT         NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, card_id)
);

CREATE INDEX idx_reviews_user_due ON reviews (user_id, next_due);

CREATE TABLE uploads (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    deck_id        UUID NOT NULL REFERENCES decks(id) ON DELETE CASCADE,
    filename       TEXT NOT NULL,
    imported_count INT  NOT NULL DEFAULT 0,
    updated_count  INT  NOT NULL DEFAULT 0,
    invalid_count  INT  NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
