ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS subscription_quota_reset_mode VARCHAR(32) NOT NULL DEFAULT 'rolling';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'groups_subscription_quota_reset_mode_check'
    ) THEN
        ALTER TABLE groups
            ADD CONSTRAINT groups_subscription_quota_reset_mode_check
            CHECK (subscription_quota_reset_mode IN ('rolling', 'until_subscription_expires'));
    END IF;
END $$;
