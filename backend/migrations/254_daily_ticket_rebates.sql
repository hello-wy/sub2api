CREATE TABLE daily_ticket_rebates (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    settlement_date DATE NOT NULL,
    daily_consumption NUMERIC(20,8) NOT NULL,
    ticket_count INTEGER NOT NULL DEFAULT 0,
    matched_rules JSONB NOT NULL DEFAULT '[]'::jsonb,
    amount_rebate NUMERIC(20,8) NOT NULL DEFAULT 0,
    status VARCHAR(16) NOT NULL CHECK (status IN ('pending', 'success', 'failed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, settlement_date)
);

CREATE INDEX idx_daily_ticket_rebates_date ON daily_ticket_rebates (settlement_date, user_id);
