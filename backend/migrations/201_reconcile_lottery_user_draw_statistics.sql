WITH draw_stats AS (
    SELECT
        d.user_id,
        COUNT(*)::BIGINT AS total_draw_attempts,
        COUNT(*) FILTER (WHERE d.prize_type <> 'none')::BIGINT AS total_wins
    FROM lottery_draws AS d
    JOIN users AS u ON u.id = d.user_id
    GROUP BY d.user_id
)
INSERT INTO lottery_user_states (
    user_id,
    total_draw_attempts,
    total_wins
)
SELECT
    user_id,
    total_draw_attempts,
    total_wins
FROM draw_stats
ON CONFLICT (user_id) DO UPDATE
SET total_draw_attempts = EXCLUDED.total_draw_attempts,
    total_wins = EXCLUDED.total_wins,
    updated_at = NOW(),
    version = lottery_user_states.version + 1
WHERE lottery_user_states.total_draw_attempts IS DISTINCT FROM EXCLUDED.total_draw_attempts
   OR lottery_user_states.total_wins IS DISTINCT FROM EXCLUDED.total_wins;
