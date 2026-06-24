-- 排行榜福利发放记录表
CREATE TABLE IF NOT EXISTS welfare_records (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL,
    user_email  VARCHAR(255) NOT NULL DEFAULT '',
    amount      DECIMAL(20, 8) NOT NULL DEFAULT 0,
    remarks     TEXT NOT NULL DEFAULT '',
    status      VARCHAR(30) NOT NULL DEFAULT 'success',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_welfare_records_user_id ON welfare_records(user_id);
CREATE INDEX IF NOT EXISTS idx_welfare_records_user_email ON welfare_records(user_email);
CREATE INDEX IF NOT EXISTS idx_welfare_records_status ON welfare_records(status);
CREATE INDEX IF NOT EXISTS idx_welfare_records_created_at ON welfare_records(created_at DESC);

-- 运营设置：排行榜福利前多少名及比例默认配置插入
INSERT INTO settings (key, value, updated_at)
VALUES ('welfare_leaderboard_rank_limit', '3', NOW())
ON CONFLICT (key) DO NOTHING;

INSERT INTO settings (key, value, updated_at)
VALUES ('welfare_leaderboard_reward_ratios', '[1.0, 0.5, 0.2]', NOW())
ON CONFLICT (key) DO NOTHING;
