ALTER TABLE lottery_user_states
    ADD COLUMN IF NOT EXISTS total_draw_attempts BIGINT NOT NULL DEFAULT 0 CHECK (total_draw_attempts >= 0),
    ADD COLUMN IF NOT EXISTS total_wins BIGINT NOT NULL DEFAULT 0 CHECK (total_wins >= 0);

INSERT INTO lottery_user_states (user_id, available_tickets, pity_misses, ticket_debt, purchase_count, total_draw_attempts, total_wins)
SELECT
    user_id,
    0,
    0,
    0,
    0,
    COUNT(*),
    COUNT(*) FILTER (WHERE prize_type <> 'none')
FROM lottery_draws
GROUP BY user_id
ON CONFLICT (user_id) DO UPDATE
SET total_draw_attempts = EXCLUDED.total_draw_attempts,
    total_wins = EXCLUDED.total_wins,
    updated_at = NOW(),
    version = lottery_user_states.version + 1;
