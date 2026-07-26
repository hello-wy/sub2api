ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS subscription_total_limit_usd DECIMAL(20,8);

ALTER TABLE user_subscriptions
    ADD COLUMN IF NOT EXISTS total_usage_usd DECIMAL(20,10) NOT NULL DEFAULT 0;

-- Preserve consumption for installations that used the temporary lifetime mode
-- before it received a dedicated total-usage column.
UPDATE user_subscriptions us
SET total_usage_usd = GREATEST(us.daily_usage_usd, us.weekly_usage_usd, us.monthly_usage_usd)
FROM groups g
WHERE us.group_id = g.id
  AND g.subscription_quota_reset_mode = 'until_subscription_expires'
  AND us.total_usage_usd = 0;
