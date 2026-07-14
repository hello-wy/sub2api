-- Migration: 174_channel_monitor_output_throughput
-- Persist successful monitor response output-token counts and end-to-end throughput.
-- Both columns stay nullable when usage is absent/invalid or duration is non-positive.

ALTER TABLE channel_monitor_histories
    ADD COLUMN IF NOT EXISTS output_tokens INTEGER,
    ADD COLUMN IF NOT EXISTS throughput_tps DOUBLE PRECISION;
