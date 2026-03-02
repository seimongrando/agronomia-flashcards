-- Migration 013: PWA push notification subscriptions.
-- Each row represents one browser subscription for one user.
-- A user may have multiple subscriptions (multiple browsers/devices).
CREATE TABLE push_subscriptions (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    endpoint   TEXT        NOT NULL,
    p256dh     TEXT        NOT NULL, -- client's ECDH public key (base64url)
    auth       TEXT        NOT NULL, -- client's auth secret (base64url)
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, endpoint)
);

CREATE INDEX idx_push_subscriptions_user_id ON push_subscriptions (user_id);
