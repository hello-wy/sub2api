INSERT INTO users (email, password_hash, balance, frozen_balance)
VALUES ('migration239@example.test', 'test', 100, 20);

INSERT INTO groups (name, rate_multiplier, image_rate_independent, image_rate_multiplier,
                    video_rate_independent, video_rate_multiplier, peak_rate_multiplier,
                    batch_image_discount_multiplier, batch_image_hold_multiplier,
                    subscription_total_limit_usd, daily_limit_usd, weekly_limit_usd, monthly_limit_usd)
VALUES ('migration239', 1.2345, true, 2.3456, true, 0.0001, 2, 0.5, 0.6, 1000, 100, 300, 800),
       ('migration239-free', 0, false, 0, false, 0, 1, 0, 0, NULL, NULL, NULL, NULL);

INSERT INTO user_group_rate_multipliers (user_id, group_id, rate_multiplier, rpm_override)
SELECT u.id, g.id, CASE WHEN g.name = 'migration239' THEN 0.0001 ELSE NULL END, 123
FROM users u CROSS JOIN groups g
WHERE u.email = 'migration239@example.test' AND g.name IN ('migration239', 'migration239-free');

INSERT INTO accounts (name, platform, type, rate_multiplier)
VALUES ('migration239', 'openai', 'apikey', 0.8);
INSERT INTO api_keys (user_id, key, name)
SELECT id, 'migration239-test-key', 'migration239' FROM users WHERE email = 'migration239@example.test';
INSERT INTO usage_logs (user_id, api_key_id, account_id, model, input_cost, output_cost,
                        total_cost, actual_cost, rate_multiplier, account_rate_multiplier)
SELECT u.id, k.id, a.id, 'migration239-model', 3, 7, 10, 12.345, 1.2345, 0.8
FROM users u JOIN api_keys k ON k.user_id = u.id CROSS JOIN accounts a
WHERE u.email = 'migration239@example.test' AND k.name = 'migration239' AND a.name = 'migration239';

INSERT INTO user_subscriptions (user_id, group_id, starts_at, expires_at, status,
                                total_usage_usd, daily_usage_usd, weekly_usage_usd, monthly_usage_usd)
SELECT u.id, g.id, NOW() - INTERVAL '1 day', NOW() + INTERVAL '30 days', 'active', 250, 20, 60, 200
FROM users u CROSS JOIN groups g
WHERE u.email = 'migration239@example.test' AND g.name = 'migration239';

INSERT INTO daily_checkin_records (user_id, checkin_date, base_reward, bonus_reward, total_reward, streak_days)
SELECT id, CURRENT_DATE, 0.01, 6, 6.01, 7 FROM users WHERE email = 'migration239@example.test';

INSERT INTO lottery_draws (user_id, request_id, prize_id, prize_label, prize_type, reward_amount, pool_version)
SELECT id, 'migration239', 'cash', '$12.35', 'balance', 12.35, 'test'
FROM users WHERE email = 'migration239@example.test';

INSERT INTO batch_image_jobs (batch_id, user_id, provider, model, item_count, estimated_cost, hold_amount,
                              group_rate_multiplier, base_unit_price, billable_unit_price, hold_unit_price,
                              pricing_snapshot_version)
SELECT 'migration239', id, 'vertex', 'test-model', 4, 2, 2.4, 2, 0.5, 0.5, 0.6, 1
FROM users WHERE email = 'migration239@example.test';
INSERT INTO batch_image_jobs (batch_id, user_id, provider, model, item_count, estimated_cost, hold_amount)
SELECT 'migration239-legacy', id, 'vertex', 'test-model', 4, 2, 2.4
FROM users WHERE email = 'migration239@example.test';

INSERT INTO settings (key, value) VALUES
('daily_checkin_reward_min', '0.01'),
('daily_checkin_reward_max', '3.5'),
('daily_checkin_reward_ranges', '[{"min":0.01,"max":3.5,"probability":1}]'),
('daily_checkin_streak_rules', '[{"threshold":7,"bonus":6.5}]'),
('lottery_prize_pool', '[{"id":"cash","type":"balance","label":"$12.35","amount":12.35,"probability":0.25,"weight":25,"eligible_for_pity":true},{"id":"subscription","type":"subscription","amount":30,"probability":0.75}]'),
('lottery_purchase_price', '30.25'),
('lottery_invitation_consumption_amount', '100.5'),
('lottery_invitation_recharge_amount', '20'),
('default_balance', '1.25'),
('default_platform_quotas', '{}'),
('auth_source_default_email_platform_quotas', '{"openai":{"daily":12.5,"weekly":50,"monthly":100}}'),
('BALANCE_RECHARGE_MULTIPLIER', '10')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;
