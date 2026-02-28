-- 007_deck_owner.up.sql
-- Tracks which professor/admin created each deck so ownership can be enforced.
-- Nullable to allow existing decks (before this migration) to have no owner;
-- admins are permitted to manage all decks regardless of created_by.

ALTER TABLE decks
    ADD COLUMN IF NOT EXISTS created_by UUID REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_decks_created_by ON decks (created_by);
