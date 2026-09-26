-- Persist live execution progress so every replica and reopened dialog sees the same run.
ALTER TABLE scheduled_test_plans ADD COLUMN IF NOT EXISTS execution_state JSONB;
