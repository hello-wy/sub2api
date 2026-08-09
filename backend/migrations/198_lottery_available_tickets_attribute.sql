-- Register the computed lottery ticket count as a read-only user attribute.
-- Existing installations that created the Chinese display name manually keep
-- their column and are mapped by the application without a duplicate field.
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
    'lottery_available_tickets',
    '剩余抽奖次数',
    '系统实时计算，不可手工修改。',
    'number',
    '[]'::jsonb,
    FALSE,
    '{"min":0}'::jsonb,
    '0',
    902,
    TRUE
WHERE NOT EXISTS (
    SELECT 1
    FROM user_attribute_definitions
    WHERE deleted_at IS NULL
      AND (key = 'lottery_available_tickets' OR (type = 'number' AND name IN ('剩余抽奖次数', '抽奖次数')))
);
