-- 004_sm2_ease.down.sql
ALTER TABLE reviews
    DROP COLUMN IF EXISTS ease_factor,
    DROP COLUMN IF EXISTS interval_days;
