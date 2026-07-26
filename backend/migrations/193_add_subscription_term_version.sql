ALTER TABLE user_subscriptions
    ADD COLUMN IF NOT EXISTS term_version BIGINT NOT NULL DEFAULT 1;

ALTER TABLE user_subscriptions
    DROP CONSTRAINT IF EXISTS user_subscriptions_term_version_positive;

ALTER TABLE user_subscriptions
    ADD CONSTRAINT user_subscriptions_term_version_positive
    CHECK (term_version > 0);
