-- Request completion, rather than billing amount, is the monitoring source of truth.
CREATE TABLE IF NOT EXISTS channel_monitor_request_outcomes (
  request_id text NOT NULL,
  group_id bigint NOT NULL,
  user_id bigint NOT NULL,
  platform text NOT NULL,
  model text NOT NULL,
  success boolean NOT NULL,
  error_category text NOT NULL DEFAULT '',
  completed_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY(request_id,user_id,group_id)
);
CREATE INDEX IF NOT EXISTS idx_channel_monitor_outcomes_completed ON channel_monitor_request_outcomes (completed_at);

ALTER TABLE channel_monitor_v2_metrics_1m ADD COLUMN IF NOT EXISTS generation_tps_sum double precision NOT NULL DEFAULT 0;
ALTER TABLE channel_monitor_v2_metrics_1m ADD COLUMN IF NOT EXISTS generation_tps_count bigint NOT NULL DEFAULT 0;
ALTER TABLE channel_monitor_v2_metrics_rollup ADD COLUMN IF NOT EXISTS generation_tps_sum double precision NOT NULL DEFAULT 0;
ALTER TABLE channel_monitor_v2_metrics_rollup ADD COLUMN IF NOT EXISTS generation_tps_count bigint NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS group_probe_configs (
  group_id bigint PRIMARY KEY REFERENCES groups(id) ON DELETE CASCADE,
  enabled boolean NOT NULL DEFAULT false,
  model text NOT NULL,
  reasoning_effort text NOT NULL DEFAULT '',
  interval_seconds integer NOT NULL DEFAULT 300 CHECK (interval_seconds BETWEEN 60 AND 86400),
  timeout_seconds integer NOT NULL DEFAULT 30 CHECK (timeout_seconds BETWEEN 5 AND 120),
  max_output_tokens integer NOT NULL DEFAULT 64 CHECK (max_output_tokens BETWEEN 16 AND 1024),
  daily_token_budget bigint NOT NULL DEFAULT 10000 CHECK (daily_token_budget BETWEEN 256 AND 1000000),
  updated_by bigint NOT NULL REFERENCES users(id),
  updated_at timestamptz NOT NULL DEFAULT now(),
  next_run_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS group_probe_runs (
  id text PRIMARY KEY,
  group_id bigint NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
  model text NOT NULL,
  status text NOT NULL CHECK (status IN ('running','success','failed','skipped')),
  checked_at timestamptz NOT NULL DEFAULT now(),
  finished_at timestamptz,
  lease_until timestamptz NOT NULL,
  latency_ms bigint,
  input_tokens bigint NOT NULL DEFAULT 0,
  output_tokens bigint NOT NULL DEFAULT 0,
  reserved_tokens bigint NOT NULL DEFAULT 0,
  estimated_cost_usd double precision,
  usage_recorded boolean NOT NULL DEFAULT false,
  error_code text NOT NULL DEFAULT '',
  scheduled boolean NOT NULL DEFAULT false
);
CREATE INDEX IF NOT EXISTS idx_group_probe_runs_group_time ON group_probe_runs(group_id, checked_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_group_probe_single_running ON group_probe_runs(group_id) WHERE status = 'running';

-- Rebuild passive rollups with the corrected success and speed definitions.
-- Existing rows remain readable during the bounded background backfill.
UPDATE channel_monitor_v2_watermarks SET backfill_cursor = now(), usage_coverage_start = now(), error_coverage_start = now() WHERE id = 1;
