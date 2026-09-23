-- Historical aggregates did not record how many durations were measured.
-- NULL deliberately means unknown, rather than treating missing timings as 0.
ALTER TABLE usage_dashboard_hourly ADD COLUMN IF NOT EXISTS duration_samples BIGINT;
ALTER TABLE usage_dashboard_daily ADD COLUMN IF NOT EXISTS duration_samples BIGINT;
