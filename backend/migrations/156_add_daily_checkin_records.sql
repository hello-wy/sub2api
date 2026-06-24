-- 用户每日签到奖励记录表。
-- 每个用户在其选定时区下每天最多一条记录，用于幂等兜底和奖励明细展示。

CREATE TABLE IF NOT EXISTS daily_checkin_records (
    id             BIGSERIAL PRIMARY KEY,
    user_id        BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    checkin_date   DATE NOT NULL,
    timezone       VARCHAR(64) NOT NULL DEFAULT 'UTC',
    base_reward    DECIMAL(20, 8) NOT NULL DEFAULT 0,
    bonus_reward   DECIMAL(20, 8) NOT NULL DEFAULT 0,
    total_reward   DECIMAL(20, 8) NOT NULL DEFAULT 0,
    streak_days    INTEGER NOT NULL DEFAULT 1,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS dailycheckinrecord_user_id_checkin_date
    ON daily_checkin_records(user_id, checkin_date);

CREATE INDEX IF NOT EXISTS dailycheckinrecord_user_id
    ON daily_checkin_records(user_id);

CREATE INDEX IF NOT EXISTS dailycheckinrecord_checkin_date
    ON daily_checkin_records(checkin_date);

CREATE INDEX IF NOT EXISTS dailycheckinrecord_created_at
    ON daily_checkin_records(created_at);
