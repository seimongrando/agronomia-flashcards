-- Migration 011: add missing performance indexes
-- Identified by query audit: these columns appear in WHERE clauses without indexes.

-- 1. uploads.deck_id — used by UploadRepo.ListByDeck (upload.go)
--    Previously only idx_uploads_user_created (user_id, created_at DESC) existed.
CREATE INDEX IF NOT EXISTS idx_uploads_deck_id
    ON uploads (deck_id, created_at DESC);

-- 2. reviews.updated_at — used by ProfessorStats active-student count (study.go)
--    The composite idx_reviews_user_updated_at cannot serve a query filtering
--    only on updated_at (leading column is user_id).
CREATE INDEX IF NOT EXISTS idx_reviews_updated_at
    ON reviews (updated_at DESC);

-- 3. decks (is_private, created_by) — used by ListPrivateByOwner (deck.go)
--    Partial index: only covers private decks, keeping index size small.
CREATE INDEX IF NOT EXISTS idx_decks_private_owner
    ON decks (created_by)
    WHERE is_private = true;
