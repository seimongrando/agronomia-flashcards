-- 004_sm2_ease.up.sql
-- Add SM-2 spaced-repetition fields to the reviews table.
-- ease_factor: the inter-repetition multiplier (EF), starts at 2.5.
-- interval_days: last computed interval in days, used as the base for the next one.
ALTER TABLE reviews
    ADD COLUMN ease_factor   NUMERIC(4,2) NOT NULL DEFAULT 2.50,
    ADD COLUMN interval_days INT          NOT NULL DEFAULT 1;
