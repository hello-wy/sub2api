-- Seed membership point attributes used by the membership discount program.
-- These are user attribute definitions, not physical columns on users.

INSERT INTO user_attribute_definitions (
    key,
    name,
    description,
    type,
    options,
    required,
    validation,
    placeholder,
    display_order,
    enabled
)
SELECT
    'loyalty_weekly_points',
    '周积分',
    '会员计划本周累计积分。每周一按服务端时区重新计算有效周期。',
    'number',
    '[]'::jsonb,
    FALSE,
    '{"min":0}'::jsonb,
    '0',
    900,
    TRUE
WHERE NOT EXISTS (
    SELECT 1
    FROM user_attribute_definitions
    WHERE key = 'loyalty_weekly_points'
      AND deleted_at IS NULL
);

INSERT INTO user_attribute_definitions (
    key,
    name,
    description,
    type,
    options,
    required,
    validation,
    placeholder,
    display_order,
    enabled
)
SELECT
    'loyalty_permanent_points',
    '永久积分',
    '会员计划永久累计积分。用于解锁长期充值折扣。',
    'number',
    '[]'::jsonb,
    FALSE,
    '{"min":0}'::jsonb,
    '0',
    901,
    TRUE
WHERE NOT EXISTS (
    SELECT 1
    FROM user_attribute_definitions
    WHERE key = 'loyalty_permanent_points'
      AND deleted_at IS NULL
);
