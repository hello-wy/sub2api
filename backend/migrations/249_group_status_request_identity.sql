-- Monitoring uses durable audio/search billing identities while preserving the
-- original gateway identity for error correlation. No money keys are changed.
ALTER TABLE channel_monitor_request_outcomes
  ADD COLUMN IF NOT EXISTS gateway_request_id text NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_channel_monitor_outcomes_gateway_request
  ON channel_monitor_request_outcomes(gateway_request_id, user_id, group_id)
  WHERE gateway_request_id <> '';
