-- Routing records and usage-log account_id values remain unchanged. This
-- association is separate from Spark's parent_account_id and business channels.
CREATE TABLE IF NOT EXISTS account_ip_logical_accounts (
    account_id BIGINT PRIMARY KEY REFERENCES accounts(id),
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS account_ip_channels (
    account_id BIGINT PRIMARY KEY REFERENCES accounts(id),
    logical_account_id BIGINT NOT NULL REFERENCES account_ip_logical_accounts(account_id),
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    disabled_at TIMESTAMPTZ,
    retired_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS account_ip_channels_logical_idx ON account_ip_channels(logical_account_id, account_id);
CREATE TABLE IF NOT EXISTS account_ip_merge_conflicts (
    id BIGSERIAL PRIMARY KEY,
    account_ids BIGINT[] NOT NULL,
    reason TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Only automatically merge prior multi-IP imports with a durable credential
-- group and identical shared settings. Email alone is never an identity key.
-- Conflicts remain separate, visible accounts, with an auditable explanation.
WITH candidates AS (
    SELECT a.*, a.credentials->>'_oauth_credential_group' AS identity,
      jsonb_build_object(
        'workspace', a.credentials->>'chatgpt_account_id',
        'user', a.credentials->>'chatgpt_user_id',
        'client', a.credentials->>'client_id',
        'credential_config', a.credentials - ARRAY['access_token','refresh_token','id_token','expires_at','expires_in','_token_version','_oauth_source_fingerprint','_oauth_source_fingerprints'],
        'extra', a.extra - ARRAY['codex_fingerprint_seed','codex_usage_updated_at','codex_primary_used_percent','codex_primary_reset_after_seconds','codex_primary_window_minutes','codex_secondary_used_percent','codex_secondary_reset_after_seconds','codex_secondary_window_minutes','codex_primary_over_secondary_percent','codex_5h_used_percent','codex_5h_reset_after_seconds','codex_5h_window_minutes','codex_5h_reset_at','codex_7d_used_percent','codex_7d_reset_after_seconds','codex_7d_window_minutes','codex_7d_reset_at'],
        'rate', a.rate_multiplier, 'expires', a.expires_at,
        'pause', a.auto_pause_on_expired,
        'groups', COALESCE((SELECT jsonb_agg(jsonb_build_array(ag.group_id, ag.priority) ORDER BY ag.group_id) FROM account_groups ag WHERE ag.account_id = a.id), '[]'::jsonb)
      ) AS settings
    FROM accounts a
    WHERE a.deleted_at IS NULL AND a.platform='openai' AND a.type='oauth'
      AND a.parent_account_id IS NULL AND a.proxy_id IS NOT NULL
      AND COALESCE(a.extra->>'proxy_mode','') <> 'random'
      AND COALESCE(a.credentials->>'_oauth_credential_group','') <> ''
      AND NOT EXISTS (SELECT 1 FROM accounts shadow WHERE shadow.parent_account_id=a.id AND shadow.deleted_at IS NULL)
), groups AS (
    SELECT identity, min(id) root_id, array_agg(id ORDER BY id) ids,
      count(*) members, count(DISTINCT proxy_id) proxies, count(DISTINCT settings) settings_count
    FROM candidates GROUP BY identity HAVING count(*) > 1
), conflicts AS (
    INSERT INTO account_ip_merge_conflicts(account_ids, reason)
    SELECT ids, 'Shared configuration differs, duplicate proxy, or more than 50 channels; automatic merge skipped'
    FROM groups WHERE settings_count <> 1 OR members <> proxies OR members > 50
), roots AS (
INSERT INTO account_ip_logical_accounts(account_id)
    SELECT root_id FROM groups WHERE settings_count=1 AND members=proxies AND members<=50
    ON CONFLICT (account_id) DO NOTHING
    RETURNING account_id
)
INSERT INTO account_ip_channels(account_id, logical_account_id, enabled, disabled_at)
SELECT c.id, r.account_id, c.schedulable, CASE WHEN NOT c.schedulable THEN NOW() END
FROM roots r JOIN groups g ON g.root_id=r.account_id JOIN candidates c ON c.identity=g.identity
ON CONFLICT (account_id) DO NOTHING;

UPDATE accounts SET name=regexp_replace(name, ' \[IP #[0-9]+\]$', '')
WHERE id IN (SELECT account_id FROM account_ip_logical_accounts);

-- Runtime recovery, quota reset and token refresh also update accounts. They
-- must not accidentally re-enable a manually paused or retired IP channel.
CREATE OR REPLACE FUNCTION enforce_account_ip_channel_state() RETURNS trigger AS $$
DECLARE channel_enabled BOOLEAN; logical_enabled BOOLEAN; retired TIMESTAMPTZ;
BEGIN
    SELECT c.enabled,l.enabled,c.retired_at INTO channel_enabled,logical_enabled,retired
    FROM account_ip_channels c JOIN account_ip_logical_accounts l ON l.account_id=c.logical_account_id
    WHERE c.account_id=NEW.id;
    IF FOUND THEN
        IF NOT channel_enabled OR NOT logical_enabled OR retired IS NOT NULL THEN NEW.schedulable=FALSE; END IF;
        IF NEW.proxy_id IS DISTINCT FROM OLD.proxy_id AND
           (OLD.schedulable OR channel_enabled OR retired IS NOT NULL) THEN
            RAISE EXCEPTION 'Fixed IP channel must be paused and drained before changing proxy';
        END IF;
        IF COALESCE(NEW.extra->>'proxy_mode','')='random' THEN RAISE EXCEPTION 'IP channels require a fixed proxy'; END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS accounts_ip_channel_state ON accounts;
CREATE TRIGGER accounts_ip_channel_state BEFORE UPDATE ON accounts
FOR EACH ROW EXECUTE FUNCTION enforce_account_ip_channel_state();
