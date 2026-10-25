-- Display order is independent of monitor identity and its historical data.
ALTER TABLE channel_monitors
    ADD COLUMN IF NOT EXISTS sort_order INTEGER NOT NULL DEFAULT 0
    CHECK (sort_order >= 0);
