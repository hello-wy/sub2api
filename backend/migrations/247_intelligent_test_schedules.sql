-- Scheduling is opt-in. Existing results, settings and queued manual tests
-- keep their original behavior; no historical tests are replayed.
CREATE TABLE IF NOT EXISTS intelligent_test_schedules (
    test_type VARCHAR(64) PRIMARY KEY REFERENCES test_settings(test_type) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT false,
    interval_minutes INTEGER NOT NULL DEFAULT 60 CHECK (interval_minutes BETWEEN 5 AND 10080),
    account_ids BIGINT[] NOT NULL DEFAULT '{}',
    requested_by BIGINT,
    next_run_at TIMESTAMPTZ,
    last_run_at TIMESTAMPTZ,
    last_error TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (cardinality(account_ids) <= 100),
    CHECK (NOT enabled OR (cardinality(account_ids) >= 1 AND requested_by IS NOT NULL AND next_run_at IS NOT NULL))
);
CREATE INDEX IF NOT EXISTS idx_intelligent_test_schedules_due
    ON intelligent_test_schedules(next_run_at, test_type) WHERE enabled;
ALTER TABLE account_tests ADD COLUMN IF NOT EXISTS scheduled BOOLEAN NOT NULL DEFAULT false;

-- Public/admin galleries fill missing legacy preview metadata in small batches.
-- Finished modern or previously rejected images leave this index immediately.
CREATE INDEX IF NOT EXISTS idx_account_tests_pelican_preview_pending
    ON account_tests(account_id, id DESC)
    WHERE test_type='pelican' AND status IN ('completed','success','suspected_degradation')
      AND COALESCE(evaluation->>'image_state','') NOT IN ('ready','sanitized','unavailable');
