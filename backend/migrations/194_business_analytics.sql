-- 194_business_analytics.sql
-- 经营分析：账号成本台账、API Key 成本比例与承载快照。三张表均独立于原始 usage_logs，
-- 使管理员即使清理用量明细后仍可保留可审计的账号成本和容量基线。

CREATE TABLE IF NOT EXISTS business_account_costs (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NULL REFERENCES accounts(id) ON DELETE SET NULL,
    group_id BIGINT NULL REFERENCES groups(id) ON DELETE SET NULL,
    cost_type VARCHAR(32) NOT NULL DEFAULT 'renewal',
    amount NUMERIC(20, 6) NOT NULL CHECK (amount > 0),
    currency VARCHAR(12) NOT NULL DEFAULT 'CNY',
	fx_rate NUMERIC(20, 8) NOT NULL DEFAULT 1 CHECK (fx_rate > 0), -- 1 原币 = fx_rate CNY
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (ends_at > starts_at)
);

CREATE INDEX IF NOT EXISTS idx_business_account_costs_period
    ON business_account_costs (starts_at, ends_at);
CREATE INDEX IF NOT EXISTS idx_business_account_costs_group_period
    ON business_account_costs (group_id, starts_at, ends_at);
CREATE INDEX IF NOT EXISTS idx_business_account_costs_account_period
    ON business_account_costs (account_id, starts_at, ends_at);

-- API Key 成本比例只维护每个账号的当前值。这里的 credits 是平台计价分，
-- credits_per_cny 表示每 1 元人民币可兑换多少计价分。
CREATE TABLE IF NOT EXISTS business_api_key_cost_rates (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL UNIQUE REFERENCES accounts(id) ON DELETE CASCADE,
    credits_per_cny NUMERIC(20, 8) NOT NULL CHECK (credits_per_cny > 0),
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS business_account_capacity_snapshots (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    captured_at TIMESTAMPTZ NOT NULL,
    observed_requests BIGINT NOT NULL DEFAULT 0,
    observed_account_cost NUMERIC(20, 6) NOT NULL DEFAULT 0,
    concurrency_max INTEGER NOT NULL DEFAULT 0,
    source VARCHAR(32) NOT NULL DEFAULT 'usage_window',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (account_id, group_id, captured_at)
);

CREATE INDEX IF NOT EXISTS idx_business_capacity_snapshots_group_captured
    ON business_account_capacity_snapshots (group_id, captured_at DESC);
CREATE INDEX IF NOT EXISTS idx_business_capacity_snapshots_account_captured
    ON business_account_capacity_snapshots (account_id, captured_at DESC);
