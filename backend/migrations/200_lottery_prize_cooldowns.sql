CREATE TABLE IF NOT EXISTS lottery_prize_cooldowns (
    prize_id VARCHAR(64) PRIMARY KEY,
    cooldown_until TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_lottery_prize_cooldowns_until
    ON lottery_prize_cooldowns (cooldown_until);
