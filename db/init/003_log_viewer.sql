-- Add trigram index for full-text search on job_logs
-- The job_logs table is created in 001_schema.sql

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS job_logs_search_trgm_idx
ON job_logs USING gin ((event || ' ' || message || ' ' || (detail_json::text)) gin_trgm_ops);
