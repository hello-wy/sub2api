-- Account and group plans share scheduling, leases and retained test history.
ALTER TABLE scheduled_test_plans ALTER COLUMN account_id DROP NOT NULL;
ALTER TABLE scheduled_test_plans ADD COLUMN IF NOT EXISTS group_id BIGINT REFERENCES groups(id) ON DELETE CASCADE;
ALTER TABLE scheduled_test_plans ADD COLUMN IF NOT EXISTS api_key_id BIGINT REFERENCES api_keys(id) ON DELETE SET NULL;
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'scheduled_test_plans_target_check' AND conrelid = 'scheduled_test_plans'::regclass) THEN
        ALTER TABLE scheduled_test_plans ADD CONSTRAINT scheduled_test_plans_target_check CHECK (
            (account_id IS NOT NULL AND group_id IS NULL AND api_key_id IS NULL) OR
            (account_id IS NULL AND group_id IS NOT NULL AND pelican_config IS NOT NULL AND auto_recover = false)
        );
    END IF;
END $$;
CREATE INDEX IF NOT EXISTS idx_stp_group_id ON scheduled_test_plans(group_id) WHERE group_id IS NOT NULL;

-- Keep account samples distinguishable from real group gateway tests.
ALTER TABLE pelican_showcase_items ADD COLUMN IF NOT EXISTS source_scope TEXT NOT NULL DEFAULT 'account' CHECK (source_scope IN ('account', 'group'));
