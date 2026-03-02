DROP INDEX IF EXISTS idx_class_decks_deck_id;
DROP INDEX IF EXISTS idx_class_members_user_id;
DROP INDEX IF EXISTS idx_classes_created_by;
DROP TABLE IF EXISTS class_decks;
DROP TABLE IF EXISTS class_members;
DROP TABLE IF EXISTS classes;
ALTER TABLE decks DROP COLUMN IF EXISTS is_private;
