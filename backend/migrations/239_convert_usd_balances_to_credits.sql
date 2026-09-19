-- 将历史网站积分从旧的 1 CNY = 10 积分口径转换为 1:1。
-- 迁移由 migrations runner 以事务方式执行，并通过 schema_migrations 保证只运行一次。

-- 用户余额（余额字段实际存储网站积分）。
UPDATE users
SET balance = balance / 10.0
WHERE balance <> 0;

-- 批量生图冻结余额也是用户积分余额的一部分，必须与可用余额同步折算。
UPDATE users
SET frozen_balance = frozen_balance / 10.0
WHERE frozen_balance IS NOT NULL
  AND frozen_balance <> 0;

UPDATE users
SET total_recharged = total_recharged / 10.0
WHERE total_recharged <> 0;

-- 用户余额相关配置也属于网站积分口径，避免新用户赠送余额和提醒阈值仍沿用旧的 10 倍数值。
UPDATE users
SET balance_notify_threshold = balance_notify_threshold / 10.0
WHERE balance_notify_threshold IS NOT NULL
  AND balance_notify_threshold <> 0;

UPDATE settings
SET value = (value::numeric / 10.0)::text, updated_at = NOW()
WHERE key IN (
  'default_balance',
  'auth_source_default_email_balance',
  'auth_source_default_linuxdo_balance',
  'auth_source_default_oidc_balance',
  'auth_source_default_wechat_balance',
  'auth_source_default_github_balance',
  'auth_source_default_google_balance',
  'auth_source_default_dingtalk_balance'
)
AND value ~ '^-?[0-9]+([.][0-9]+)?$'
AND value::numeric <> 0;

UPDATE settings
SET value = (value::numeric / 10.0)::text, updated_at = NOW()
WHERE key = 'balance_low_notify_threshold'
  AND value ~ '^-?[0-9]+([.][0-9]+)?$'
  AND value::numeric <> 0;

-- 默认平台配额（全局及各登录来源）与 user_platform_quotas 使用同一积分口径。
UPDATE settings AS s
SET value = (
  SELECT COALESCE(jsonb_object_agg(platform, (
    SELECT COALESCE(jsonb_object_agg(k,
      CASE
        WHEN k IN ('daily', 'weekly', 'monthly') AND jsonb_typeof(v) = 'number'
          THEN to_jsonb((v #>> '{}')::numeric / 10.0)
        ELSE v
      END
    ), '{}'::jsonb)
    FROM jsonb_each(platform_value) AS quota(k, v)
  )), '{}'::jsonb)::text
  FROM jsonb_each(s.value::jsonb) AS platforms(platform, platform_value)
), updated_at = NOW()
WHERE key IN (
  'default_platform_quotas',
  'auth_source_default_email_platform_quotas',
  'auth_source_default_linuxdo_platform_quotas',
  'auth_source_default_oidc_platform_quotas',
  'auth_source_default_wechat_platform_quotas',
  'auth_source_default_github_platform_quotas',
  'auth_source_default_google_platform_quotas',
  'auth_source_default_dingtalk_platform_quotas'
)
AND value IS JSON OBJECT;

-- 通过可配置分组倍率转换用户售价；网关不附加固定的 0.1 系数。
-- 多保留一位小数，避免旧倍率 0.0001 / 10 被原 decimal(10,4) 四舍五入成免费。
ALTER TABLE groups
    ALTER COLUMN rate_multiplier TYPE DECIMAL(11,5),
    ALTER COLUMN image_rate_multiplier TYPE DECIMAL(11,5),
    ALTER COLUMN video_rate_multiplier TYPE DECIMAL(11,5);
ALTER TABLE user_group_rate_multipliers
    ALTER COLUMN rate_multiplier TYPE DECIMAL(11,5);
ALTER TABLE usage_logs
    ALTER COLUMN rate_multiplier TYPE DECIMAL(11,5);
ALTER TABLE batch_image_jobs
    ALTER COLUMN group_rate_multiplier TYPE DECIMAL(11,5);

UPDATE groups
SET rate_multiplier = rate_multiplier / 10.0,
    image_rate_multiplier = image_rate_multiplier / 10.0,
    video_rate_multiplier = video_rate_multiplier / 10.0,
    updated_at = NOW();

-- 用户专属倍率覆盖分组倍率，必须单独折算；NULL 仍表示沿用分组设置。
UPDATE user_group_rate_multipliers
SET rate_multiplier = rate_multiplier / 10.0, updated_at = NOW()
WHERE rate_multiplier IS NOT NULL;

-- 高峰倍率、批量折扣/冻结比例、账号成本倍率、模型基础单价保持不变。

-- 分组订阅额度及已使用额度均采用相同积分口径。
UPDATE groups
SET subscription_total_limit_usd = subscription_total_limit_usd / 10.0
WHERE subscription_total_limit_usd IS NOT NULL;
UPDATE groups
SET daily_limit_usd = daily_limit_usd / 10.0
WHERE daily_limit_usd IS NOT NULL;
UPDATE groups
SET weekly_limit_usd = weekly_limit_usd / 10.0
WHERE weekly_limit_usd IS NOT NULL;
UPDATE groups
SET monthly_limit_usd = monthly_limit_usd / 10.0
WHERE monthly_limit_usd IS NOT NULL;

UPDATE user_subscriptions
SET daily_usage_usd = daily_usage_usd / 10.0,
    weekly_usage_usd = weekly_usage_usd / 10.0,
    monthly_usage_usd = monthly_usage_usd / 10.0,
    total_usage_usd = total_usage_usd / 10.0
WHERE status = 'active'
  AND starts_at <= NOW()
  AND expires_at > NOW();

UPDATE user_platform_quotas
SET daily_limit_usd = daily_limit_usd / 10.0,
    weekly_limit_usd = weekly_limit_usd / 10.0,
    monthly_limit_usd = monthly_limit_usd / 10.0,
    daily_usage_usd = daily_usage_usd / 10.0,
    weekly_usage_usd = weekly_usage_usd / 10.0,
    monthly_usage_usd = monthly_usage_usd / 10.0
WHERE deleted_at IS NULL;

-- API Key 级用户配额同样是网站积分单位（字段名保留兼容）。
UPDATE api_keys
SET quota = quota / 10.0,
    quota_used = quota_used / 10.0,
    rate_limit_5h = rate_limit_5h / 10.0,
    rate_limit_1d = rate_limit_1d / 10.0,
    rate_limit_7d = rate_limit_7d / 10.0,
    usage_5h = usage_5h / 10.0,
    usage_1d = usage_1d / 10.0,
    usage_7d = usage_7d / 10.0
WHERE quota <> 0
   OR quota_used <> 0
   OR rate_limit_5h <> 0
   OR rate_limit_1d <> 0
   OR rate_limit_7d <> 0
   OR usage_5h <> 0
   OR usage_1d <> 0
   OR usage_7d <> 0;

-- 历史充值订单中的余额到账金额（支付金额 pay_amount 保持原支付币种）。
UPDATE payment_orders
SET amount = amount / 10.0
WHERE order_type = 'balance'
  AND amount <> 0;
UPDATE payment_orders
SET refund_amount = refund_amount / 10.0
WHERE order_type = 'balance'
  AND refund_amount <> 0;

-- 余额兑换码的 value 是到账积分；支付币种金额仍保留在 payment_orders.pay_amount。
UPDATE redeem_codes
SET value = value / 10.0
WHERE type = 'balance'
  AND value <> 0;

UPDATE promo_codes
SET bonus_amount = bonus_amount / 10.0
WHERE bonus_amount <> 0;
UPDATE promo_code_usages
SET bonus_amount = bonus_amount / 10.0
WHERE bonus_amount <> 0;

-- 历史奖励、返利、抽奖及余额流水均使用网站积分口径。
UPDATE daily_checkin_records
SET base_reward = base_reward / 10.0,
    bonus_reward = bonus_reward / 10.0,
    total_reward = total_reward / 10.0;

UPDATE welfare_records
SET amount = amount / 10.0;

UPDATE lottery_draws
SET reward_amount = reward_amount / 10.0
WHERE prize_type = 'balance';
UPDATE lottery_draws
SET prize_label = '$' || trim_scale((substring(prize_label FROM 2))::numeric / 10.0)::text
WHERE prize_type = 'balance'
  AND prize_label ~ '^\$[0-9]+(\.[0-9]+)?$';

UPDATE user_affiliate_ledger
SET amount = amount / 10.0;

UPDATE user_affiliates
SET aff_quota = aff_quota / 10.0,
    aff_frozen_quota = aff_frozen_quota / 10.0,
    aff_history_quota = aff_history_quota / 10.0;

UPDATE balance_transactions
SET amount = amount / 10.0,
    balance_before = CASE WHEN balance_before IS NULL THEN NULL ELSE balance_before / 10.0 END,
    balance_after = CASE WHEN balance_after IS NULL THEN NULL ELSE balance_after / 10.0 END;

-- 用户消费记录与批量生图费用中的 actual/user cost 使用新的积分单位。
UPDATE usage_logs
SET actual_cost = actual_cost / 10.0;

-- 明细及 total_cost/account_stats_cost 保留原始成本；历史用户售价倍率
-- 与 actual_cost 一起折算，继续满足“基础价格 × 用户倍率 = 实际扣费”。
UPDATE usage_logs
SET rate_multiplier = rate_multiplier / 10.0;

-- 计费对账和历史聚合中的用户实际扣费同步折算；total_cost/account_cost 仍保留原始成本语义。
UPDATE billing_usage_entries
SET delta_usd = delta_usd / 10.0
WHERE delta_usd <> 0;

UPDATE usage_dashboard_hourly
SET actual_cost = actual_cost / 10.0
WHERE actual_cost <> 0;

UPDATE usage_dashboard_daily
SET actual_cost = actual_cost / 10.0
WHERE actual_cost <> 0;

UPDATE usage_group_daily_rollups
SET actual_cost = actual_cost / 10.0
WHERE actual_cost <> 0;

-- 旧版无单价快照的在途任务按提交时预计金额补齐快照，避免迁移后
-- 结算再次从未折算的模型基础价取价。以下金额仍为旧单位，随后统一折算。
UPDATE batch_image_jobs
SET billable_unit_price = estimated_cost / item_count,
    hold_unit_price = COALESCE(hold_amount, estimated_cost) / item_count,
    pricing_snapshot_version = 1
WHERE pricing_snapshot_version = 0
  AND item_count > 0;

UPDATE batch_image_jobs
SET group_rate_multiplier = group_rate_multiplier / 10.0,
    billable_unit_price = billable_unit_price / 10.0,
    hold_unit_price = hold_unit_price / 10.0,
    estimated_cost = estimated_cost / 10.0,
    hold_amount = CASE WHEN hold_amount IS NULL THEN NULL ELSE hold_amount / 10.0 END,
    actual_cost = CASE WHEN actual_cost IS NULL THEN NULL ELSE actual_cost / 10.0 END;

UPDATE batch_image_items
SET billed_amount = billed_amount / 10.0
WHERE billed_amount IS NOT NULL
  AND billed_amount <> 0;

-- 现有签到配置、抽奖配置和抽奖门槛按旧积分口径折算，保留管理员自定义值。
UPDATE settings
SET value = (value::numeric / 10.0)::text, updated_at = NOW()
WHERE key = 'daily_checkin_reward_min'
  AND value ~ '^-?[0-9]+([.][0-9]+)?$';
UPDATE settings
SET value = (value::numeric / 10.0)::text, updated_at = NOW()
WHERE key = 'daily_checkin_reward_max'
  AND value ~ '^-?[0-9]+([.][0-9]+)?$';
UPDATE settings
SET value = (
  SELECT COALESCE(jsonb_agg(
    jsonb_set(
      jsonb_set(range_item, '{min}', to_jsonb((range_item->>'min')::numeric / 10.0), true),
      '{max}', to_jsonb((range_item->>'max')::numeric / 10.0), true
    )
  ), '[]'::jsonb)::text
  FROM jsonb_array_elements(value::jsonb) AS r(range_item)
), updated_at = NOW()
WHERE key = 'daily_checkin_reward_ranges'
  AND value IS JSON ARRAY;
UPDATE settings
SET value = (
  SELECT COALESCE(jsonb_agg(
    jsonb_set(rule_item, '{bonus}', to_jsonb((rule_item->>'bonus')::numeric / 10.0), true)
  ), '[]'::jsonb)::text
  FROM jsonb_array_elements(value::jsonb) AS r(rule_item)
), updated_at = NOW()
WHERE key = 'daily_checkin_streak_rules'
  AND value IS JSON ARRAY;

-- 系统当前保存数组；同时兼容曾导入的 {"prizes": [...]} 格式。
WITH pools AS (
  SELECT key, value::jsonb AS original,
         CASE WHEN value IS JSON ARRAY THEN value::jsonb ELSE value::jsonb->'prizes' END AS prizes
  FROM settings
  WHERE key = 'lottery_prize_pool'
    AND (value IS JSON ARRAY OR value IS JSON OBJECT)
), converted AS (
  SELECT key, original, (
    SELECT COALESCE(jsonb_agg(
      CASE WHEN prize->>'type' = 'balance' THEN
        jsonb_set(
          jsonb_set(prize, '{amount}', to_jsonb((prize->>'amount')::numeric / 10.0)),
          '{label}', to_jsonb('$' || trim_scale((prize->>'amount')::numeric / 10.0)::text)
        )
      ELSE prize END ORDER BY ordinal
    ), '[]'::jsonb)
    FROM jsonb_array_elements(prizes) WITH ORDINALITY AS p(prize, ordinal)
  ) AS prizes
  FROM pools
  WHERE jsonb_typeof(prizes) = 'array'
)
UPDATE settings AS s
SET value = CASE WHEN jsonb_typeof(c.original) = 'array' THEN c.prizes
                 ELSE jsonb_set(c.original, '{prizes}', c.prizes) END::text,
    updated_at = NOW()
FROM converted AS c
WHERE s.key = c.key;

-- The invitation recharge threshold is a real CNY payment amount and must
-- remain unchanged. Consumption and ticket purchase price are site credits.
UPDATE settings
SET value = (value::numeric / 10.0)::text, updated_at = NOW()
WHERE key = 'lottery_invitation_consumption_amount'
  AND value ~ '^-?[0-9]+([.][0-9]+)?$'
  AND value::numeric <> 0;
UPDATE settings
SET value = (value::numeric / 10.0)::text, updated_at = NOW()
WHERE key = 'lottery_purchase_price'
  AND value ~ '^-?[0-9]+([.][0-9]+)?$'
  AND value::numeric <> 0;

INSERT INTO settings (key, value)
VALUES ('BALANCE_RECHARGE_MULTIPLIER', '1')
ON CONFLICT (key) DO UPDATE SET value = '1';

-- 记录一次性单位迁移，便于运维审计和回溯。
INSERT INTO audit_logs (actor_user_id, actor_email, actor_role, action, method, path, request_body, extra)
VALUES (
  NULL,
  'system',
  'system',
  'SITE_CREDITS_UNIT_MIGRATION',
  'MIGRATION',
  '239_convert_usd_balances_to_credits.sql',
  '',
  '{"from_scale": 10, "to_scale": 1, "user_facing_symbol": "$", "conversion": "divide_by_10"}'::jsonb
);
