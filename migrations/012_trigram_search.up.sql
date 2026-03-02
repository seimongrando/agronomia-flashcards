-- Migration 012: GIN trigram indexes for efficient ILIKE card search.
--
-- The card search in ListByDeckPaged uses ILIKE '%term%' on (question, topic).
-- Without trigram indexes PostgreSQL performs a sequential scan of the cards
-- table for every search request. The pg_trgm extension allows GIN indexes to
-- accelerate both leading- and trailing-wildcard ILIKE patterns.
--
-- Safe to apply on a live database: CREATE INDEX CONCURRENTLY would be
-- preferable in large-data scenarios, but golang-migrate runs each migration
-- in a transaction where CONCURRENTLY is not permitted. The indexes are small
-- in typical academic deployments and creation is near-instant.

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS idx_cards_question_trgm
    ON cards USING gin (question gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_cards_topic_trgm
    ON cards USING gin (topic gin_trgm_ops);
