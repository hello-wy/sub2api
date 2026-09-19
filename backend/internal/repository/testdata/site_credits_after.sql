DO $$
DECLARE
    u users%ROWTYPE;
    g groups%ROWTYPE;
    s user_subscriptions%ROWTYPE;
    c daily_checkin_records%ROWTYPE;
    d lottery_draws%ROWTYPE;
    b batch_image_jobs%ROWTYPE;
    pool jsonb;
BEGIN
    SELECT * INTO STRICT u FROM users WHERE email = 'migration239@example.test';
    ASSERT u.balance = 10 AND u.frozen_balance = 2, 'wallet conversion';
    SELECT * INTO STRICT g FROM groups WHERE name = 'migration239';
    ASSERT g.rate_multiplier = 0.12345 AND g.image_rate_multiplier = 0.23456
       AND g.video_rate_multiplier = 0.00001, 'all configurable group rates, including precision';
    ASSERT g.peak_rate_multiplier = 2 AND g.batch_image_discount_multiplier = 0.5
       AND g.batch_image_hold_multiplier = 0.6, 'relative factors unchanged';
    ASSERT (SELECT rate_multiplier = 0.00001 AND rpm_override = 123
            FROM user_group_rate_multipliers WHERE user_id = u.id AND group_id = g.id), 'user override';
    ASSERT (SELECT rate_multiplier = 0 FROM groups WHERE name = 'migration239-free'), 'free group';
    ASSERT (SELECT rate_multiplier IS NULL AND rpm_override = 123
            FROM user_group_rate_multipliers WHERE user_id = u.id AND group_id != g.id), 'inherited user rate';
    ASSERT (SELECT actual_cost = 1.2345 AND rate_multiplier = 0.12345 AND total_cost = 10
               AND input_cost = 3 AND output_cost = 7 AND account_rate_multiplier = 0.8
            FROM usage_logs WHERE user_id = u.id AND model = 'migration239-model'), 'actual charge changes, base costs do not';
    ASSERT (SELECT rate_multiplier = 0.8 FROM accounts WHERE name = 'migration239'), 'supplier multiplier unchanged';
    SELECT * INTO STRICT s FROM user_subscriptions WHERE user_id = u.id AND group_id = g.id;
    ASSERT g.subscription_total_limit_usd = 100 AND s.total_usage_usd = 25
       AND g.subscription_total_limit_usd - s.total_usage_usd = 75
       AND s.total_usage_usd / g.subscription_total_limit_usd = 0.25, 'subscription value and ratio';
    ASSERT g.daily_limit_usd = 10 AND g.weekly_limit_usd = 30 AND g.monthly_limit_usd = 80
       AND s.daily_usage_usd = 2 AND s.weekly_usage_usd = 6 AND s.monthly_usage_usd = 20, 'period quotas';
    SELECT * INTO STRICT c FROM daily_checkin_records WHERE user_id = u.id;
    ASSERT c.base_reward = 0.001 AND c.bonus_reward = 0.6 AND c.total_reward = 0.601
       AND c.streak_days = 7, 'checkin history';
    SELECT * INTO STRICT d FROM lottery_draws WHERE user_id = u.id;
    ASSERT d.reward_amount = 1.235 AND d.prize_label = '$1.235', 'lottery history and fractional label';
    SELECT * INTO STRICT b FROM batch_image_jobs WHERE batch_id = 'migration239';
    ASSERT b.group_rate_multiplier = 0.2 AND b.base_unit_price = 0.5
       AND b.billable_unit_price = 0.05 AND b.hold_unit_price = 0.06
       AND b.estimated_cost = 0.2 AND b.hold_amount = 0.24, 'pending batch snapshot';
    SELECT * INTO STRICT b FROM batch_image_jobs WHERE batch_id = 'migration239-legacy';
    ASSERT b.pricing_snapshot_version = 1 AND b.billable_unit_price = 0.05
       AND b.hold_unit_price = 0.06, 'legacy batch must not settle using old units';
    ASSERT (SELECT value::numeric = 0.001 FROM settings WHERE key = 'daily_checkin_reward_min'), 'checkin min';
    ASSERT (SELECT value::numeric = 0.35 FROM settings WHERE key = 'daily_checkin_reward_max'), 'checkin max';
    ASSERT (SELECT value::jsonb = '[{"min":0.001,"max":0.35,"probability":1}]'::jsonb
            FROM settings WHERE key = 'daily_checkin_reward_ranges'), 'custom checkin range';
    ASSERT (SELECT value::jsonb = '[{"threshold":7,"bonus":0.65}]'::jsonb
            FROM settings WHERE key = 'daily_checkin_streak_rules'), 'custom streak bonus';
    SELECT value::jsonb INTO STRICT pool FROM settings WHERE key = 'lottery_prize_pool';
    IF jsonb_typeof(pool) = 'object' THEN
        ASSERT pool->>'version' = 'keep', 'pool metadata';
        pool := pool->'prizes';
    END IF;
    ASSERT pool = '[{"id":"cash","type":"balance","label":"$1.235","amount":1.235,"probability":0.25,"weight":25,"eligible_for_pity":true},{"id":"subscription","type":"subscription","amount":30,"probability":0.75}]'::jsonb,
           'prizes converted, odds and non-balance rewards preserved';
    ASSERT (SELECT value::numeric = 3.025 FROM settings WHERE key = 'lottery_purchase_price'), 'fractional ticket price';
    ASSERT (SELECT value::numeric = 10.05 FROM settings WHERE key = 'lottery_invitation_consumption_amount'), 'spending threshold';
    ASSERT (SELECT value::numeric = 20 FROM settings WHERE key = 'lottery_invitation_recharge_amount'), 'CNY threshold unchanged';
    ASSERT (SELECT value::numeric = 0.125 FROM settings WHERE key = 'default_balance'), 'fractional setting';
    ASSERT (SELECT value::jsonb = '{}'::jsonb FROM settings WHERE key = 'default_platform_quotas'), 'empty quotas';
    ASSERT (SELECT value::jsonb = '{"openai":{"daily":1.25,"weekly":5,"monthly":10}}'::jsonb
            FROM settings WHERE key = 'auth_source_default_email_platform_quotas'), 'default quotas';
    ASSERT (SELECT value = '1' FROM settings WHERE key = 'BALANCE_RECHARGE_MULTIPLIER'), 'recharge rate';
    ASSERT EXISTS (SELECT 1 FROM audit_logs WHERE action = 'SITE_CREDITS_UNIT_MIGRATION'), 'audit record';
END $$;
