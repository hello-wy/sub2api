-- Rename loyalty point attribute descriptions to membership wording.

UPDATE user_attribute_definitions
SET description = '会员计划本周累计积分。每周一按服务端时区重新计算有效周期。',
    updated_at = NOW()
WHERE key = 'loyalty_weekly_points'
  AND deleted_at IS NULL
  AND description = '忠诚计划本周累计积分。每周一按服务端时区重新计算有效周期。';

UPDATE user_attribute_definitions
SET description = '会员计划永久累计积分。用于解锁长期充值折扣。',
    updated_at = NOW()
WHERE key = 'loyalty_permanent_points'
  AND deleted_at IS NULL
  AND description = '忠诚计划永久累计积分。用于解锁长期充值折扣。';
