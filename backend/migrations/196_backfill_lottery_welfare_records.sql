-- Every lottery result, including a non-winning draw, belongs in the welfare
-- audit trail. The source key keeps this backfill and future writes idempotent.
INSERT INTO welfare_records (
    user_id,
    user_email,
    amount,
    remarks,
    status,
    source_type,
    source_id,
    reward_type,
    reward_ref
)
SELECT
    draws.user_id,
    users.email,
    CASE WHEN draws.prize_type = 'balance' THEN draws.reward_amount ELSE 0 END,
    '抽奖奖励 #' || draws.id::text,
    'success',
    'lottery_draw',
    draws.id::text,
    draws.prize_type,
    NULLIF(draws.reward_ref, '')
FROM lottery_draws AS draws
JOIN users ON users.id = draws.user_id
LEFT JOIN welfare_records AS records
    ON records.source_type = 'lottery_draw'
   AND records.source_id = draws.id::text
WHERE records.id IS NULL
ON CONFLICT (source_type, source_id) WHERE source_type IS NOT NULL AND source_id IS NOT NULL DO NOTHING;
