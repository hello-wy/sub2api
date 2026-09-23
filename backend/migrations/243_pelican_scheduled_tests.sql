-- NULL configuration preserves existing connectivity tests.
ALTER TABLE scheduled_test_plans ADD COLUMN IF NOT EXISTS pelican_config JSONB;
ALTER TABLE scheduled_test_plans ADD COLUMN IF NOT EXISTS running_until TIMESTAMPTZ;
ALTER TABLE scheduled_test_results ADD COLUMN IF NOT EXISTS pelican_config JSONB;
CREATE UNIQUE INDEX IF NOT EXISTS idx_stp_one_pelican_per_account
    ON scheduled_test_plans(account_id) WHERE pelican_config IS NOT NULL;
ALTER TABLE scheduled_test_plans ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;
