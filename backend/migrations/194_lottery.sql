-- Lottery state is deliberately ledger-backed: an aggregate counter alone cannot
-- represent expiry, source attribution, refunds, or an auditable draw history.
CREATE TABLE IF NOT EXISTS lottery_user_states (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    available_tickets INTEGER NOT NULL DEFAULT 0 CHECK (available_tickets >= 0),
    pity_misses INTEGER NOT NULL DEFAULT 0 CHECK (pity_misses >= 0 AND pity_misses <= 4),
    ticket_debt INTEGER NOT NULL DEFAULT 0 CHECK (ticket_debt >= 0),
    purchase_business_date DATE,
    purchase_count INTEGER NOT NULL DEFAULT 0 CHECK (purchase_count >= 0),
    version BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS lottery_ticket_ledger (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    delta INTEGER NOT NULL,
    remaining INTEGER NOT NULL DEFAULT 0 CHECK (remaining >= 0),
    source_type VARCHAR(32) NOT NULL,
    source_ref VARCHAR(128) NOT NULL,
    source_order_id BIGINT REFERENCES payment_orders(id),
    business_date DATE,
    reward_tier INTEGER,
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (source_type, source_ref)
);
CREATE INDEX IF NOT EXISTS idx_lottery_ticket_ledger_available
    ON lottery_ticket_ledger (user_id, expires_at, id)
    WHERE remaining > 0 AND revoked_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_lottery_recharge_tier
    ON lottery_ticket_ledger (user_id, business_date, reward_tier)
    WHERE source_type = 'recharge';

CREATE TABLE IF NOT EXISTS lottery_draws (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    request_id VARCHAR(128) NOT NULL,
    prize_id VARCHAR(32) NOT NULL,
    prize_label VARCHAR(64) NOT NULL,
    prize_type VARCHAR(24) NOT NULL,
    reward_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    pool_version VARCHAR(32) NOT NULL,
    is_guaranteed BOOLEAN NOT NULL DEFAULT FALSE,
    reward_ref VARCHAR(128),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, request_id)
);
CREATE INDEX IF NOT EXISTS idx_lottery_draws_user_created ON lottery_draws (user_id, created_at DESC, id DESC);

-- Wallet history is separate from payment orders so gifts never become revenue.
CREATE TABLE IF NOT EXISTS balance_transactions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    transaction_type VARCHAR(48) NOT NULL,
    amount NUMERIC(20,8) NOT NULL,
    balance_before NUMERIC(20,8),
    balance_after NUMERIC(20,8),
    source_type VARCHAR(48) NOT NULL,
    source_id VARCHAR(128) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (source_type, source_id)
);
CREATE INDEX IF NOT EXISTS idx_balance_transactions_user_created ON balance_transactions (user_id, created_at DESC, id DESC);

ALTER TABLE redeem_codes ADD COLUMN IF NOT EXISTS owner_user_id BIGINT REFERENCES users(id);
CREATE INDEX IF NOT EXISTS idx_redeem_codes_owner_user_id ON redeem_codes (owner_user_id) WHERE owner_user_id IS NOT NULL;

ALTER TABLE welfare_records ADD COLUMN IF NOT EXISTS source_type VARCHAR(48);
ALTER TABLE welfare_records ADD COLUMN IF NOT EXISTS source_id VARCHAR(128);
ALTER TABLE welfare_records ADD COLUMN IF NOT EXISTS reward_type VARCHAR(48);
ALTER TABLE welfare_records ADD COLUMN IF NOT EXISTS reward_ref VARCHAR(128);
CREATE UNIQUE INDEX IF NOT EXISTS uq_welfare_records_source
    ON welfare_records (source_type, source_id)
    WHERE source_type IS NOT NULL AND source_id IS NOT NULL;
