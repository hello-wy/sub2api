-- Configure how long a user must wait before purchasing the same subscription plan again.
ALTER TABLE subscription_plans
    ADD COLUMN IF NOT EXISTS repurchase_cooldown_hours INTEGER NOT NULL DEFAULT 0;

ALTER TABLE subscription_plans
    DROP CONSTRAINT IF EXISTS subscription_plans_repurchase_cooldown_nonnegative;

ALTER TABLE subscription_plans
    ADD CONSTRAINT subscription_plans_repurchase_cooldown_nonnegative
    CHECK (repurchase_cooldown_hours >= 0);

CREATE INDEX IF NOT EXISTS payment_orders_user_plan_completed_at_idx
    ON payment_orders (user_id, plan_id, completed_at DESC)
    WHERE order_type = 'subscription' AND completed_at IS NOT NULL;
