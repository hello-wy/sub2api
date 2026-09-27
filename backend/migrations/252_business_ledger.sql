-- Independent management ledger; financial snapshots survive business cleanup.
CREATE TABLE IF NOT EXISTS business_ledger_config (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    enabled_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reporting_timezone TEXT NOT NULL DEFAULT 'Asia/Shanghai'
);
INSERT INTO business_ledger_config (id) VALUES (1) ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS business_events (
    id BIGSERIAL PRIMARY KEY,
    source_key TEXT NOT NULL UNIQUE,
    event_type TEXT NOT NULL,
    transaction_id BIGINT NOT NULL DEFAULT txid_current(),
    user_id BIGINT,
    occurred_at TIMESTAMPTZ NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    payload JSONB NOT NULL CHECK (jsonb_typeof(payload) = 'object'),
    actor_id BIGINT,
    reverses_id BIGINT
);
CREATE INDEX IF NOT EXISTS business_events_transaction ON business_events (transaction_id, id);
CREATE INDEX IF NOT EXISTS business_events_user ON business_events (user_id, id);
CREATE INDEX IF NOT EXISTS business_events_time ON business_events (occurred_at, id);

CREATE TABLE IF NOT EXISTS business_cost_pools (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    supplier TEXT NOT NULL DEFAULT '',
    unit TEXT NOT NULL DEFAULT 'upstream_credit',
    mode TEXT NOT NULL CHECK (mode IN ('prepaid', 'postpaid', 'fixed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    archived_at TIMESTAMPTZ
);
CREATE TABLE IF NOT EXISTS business_cost_bindings (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL,
    pool_id BIGINT NOT NULL REFERENCES business_cost_pools(id),
    effective_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (account_id, effective_at)
);
CREATE TABLE IF NOT EXISTS business_cost_rules (
    id BIGSERIAL PRIMARY KEY,
    pool_id BIGINT NOT NULL REFERENCES business_cost_pools(id),
    model TEXT NOT NULL DEFAULT '*',
    service_tier TEXT NOT NULL DEFAULT '',
    image_size TEXT NOT NULL DEFAULT '',
    video_resolution TEXT NOT NULL DEFAULT '',
    effective_at TIMESTAMPTZ NOT NULL,
    basis TEXT NOT NULL CHECK (basis IN ('account_stats', 'tokens', 'request', 'image', 'video_second')),
    unit_price NUMERIC(24,10) NOT NULL DEFAULT 0 CHECK (unit_price >= 0),
    input_price NUMERIC(24,10) NOT NULL DEFAULT 0 CHECK (input_price >= 0),
    output_price NUMERIC(24,10) NOT NULL DEFAULT 0 CHECK (output_price >= 0),
    cache_read_price NUMERIC(24,10) NOT NULL DEFAULT 0 CHECK (cache_read_price >= 0),
    cache_write_price NUMERIC(24,10) NOT NULL DEFAULT 0 CHECK (cache_write_price >= 0),
    cache_write_1h_price NUMERIC(24,10) NOT NULL DEFAULT 0 CHECK (cache_write_1h_price >= 0),
    cny_per_unit NUMERIC(24,10) NOT NULL DEFAULT 0 CHECK (cny_per_unit >= 0),
    quality TEXT NOT NULL CHECK (quality IN ('estimated', 'contract')),
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (pool_id, model, service_tier, image_size, video_resolution, effective_at)
);
CREATE TABLE IF NOT EXISTS business_projection_state (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    revision BIGINT NOT NULL DEFAULT 0,
    event_count BIGINT NOT NULL DEFAULT 0,
    state JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_error TEXT NOT NULL DEFAULT ''
);
INSERT INTO business_projection_state (id) VALUES (1) ON CONFLICT DO NOTHING;
CREATE TABLE IF NOT EXISTS business_projection_processed (
    event_id BIGINT PRIMARY KEY REFERENCES business_events(id)
);
CREATE TABLE IF NOT EXISTS business_ledger_entries (
    id TEXT PRIMARY KEY,
    event_id BIGINT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    kind TEXT NOT NULL,
    user_id BIGINT,
    account_id BIGINT,
    group_id BIGINT,
    model TEXT NOT NULL DEFAULT '',
    plan_id BIGINT,
    amount_cny NUMERIC(24,8),
    credits NUMERIC(24,8) NOT NULL DEFAULT 0,
    quality TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'
);
CREATE INDEX IF NOT EXISTS business_ledger_entries_time ON business_ledger_entries (occurred_at, kind);
CREATE INDEX IF NOT EXISTS business_ledger_entries_group ON business_ledger_entries (group_id, occurred_at);

CREATE OR REPLACE FUNCTION business_event_emit(k TEXT, t TEXT, u BIGINT, at_time TIMESTAMPTZ, data JSONB)
RETURNS VOID LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO business_events (source_key, event_type, user_id, occurred_at, payload)
    VALUES (k, t, u, COALESCE(at_time, clock_timestamp()), data)
    ON CONFLICT (source_key) DO NOTHING;
END $$;

CREATE OR REPLACE FUNCTION business_capture_wallet() RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE prior NUMERIC := 0; prior_frozen NUMERIC := 0;
BEGIN
    IF TG_OP = 'UPDATE' THEN prior := OLD.balance; prior_frozen := COALESCE(OLD.frozen_balance,0); END IF;
    IF NEW.balance = prior AND COALESCE(NEW.frozen_balance,0) = prior_frozen THEN RETURN NEW; END IF;
    PERFORM business_event_emit('wallet:' || NEW.id || ':' || nextval('business_events_id_seq'),
        CASE WHEN TG_OP = 'INSERT' THEN 'signup_wallet' ELSE 'wallet' END, NEW.id, clock_timestamp(),
        jsonb_build_object('before',prior,'after',NEW.balance,'frozen_before',prior_frozen,
            'frozen_after',COALESCE(NEW.frozen_balance,0),'user_name',NEW.username,'role',NEW.role,'context',NULLIF(current_setting('sub2api.business_wallet',true),'')::jsonb));
    RETURN NEW;
END $$;
CREATE OR REPLACE TRIGGER business_wallet AFTER INSERT OR UPDATE OF balance, frozen_balance ON users
FOR EACH ROW EXECUTE FUNCTION business_capture_wallet();

-- Capture domain evidence in the same transaction. The projector only matches
-- a wallet movement to evidence from that exact transaction, never heuristics.
CREATE OR REPLACE FUNCTION business_capture_source() RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE data JSONB; previous JSONB := '{}'; uid BIGINT;
BEGIN
    data := to_jsonb(NEW);
    IF TG_OP = 'UPDATE' THEN previous := to_jsonb(OLD); END IF;
    uid := COALESCE((data->>'user_id')::BIGINT,(data->>'used_by')::BIGINT);
    IF TG_TABLE_NAME = 'payment_orders' THEN
        IF TG_OP = 'UPDATE' AND NEW.status IS NOT DISTINCT FROM OLD.status AND NEW.refund_amount IS NOT DISTINCT FROM OLD.refund_amount THEN RETURN NEW; END IF;
        data := jsonb_build_object('id',NEW.id,'user_id',NEW.user_id,'amount',NEW.amount,
          'pay_amount',NEW.pay_amount,'payment_type',NEW.payment_type,'order_type',NEW.order_type,
          'status',NEW.status,'paid_at',NEW.paid_at,'refund_at',NEW.refund_at,
          'refund_amount',NEW.refund_amount,'plan_id',NEW.plan_id,
          'subscription_group_id',NEW.subscription_group_id,'subscription_days',NEW.subscription_days,
          'currency',COALESCE(NEW.provider_snapshot->>'currency','CNY'),
          'loyalty',NEW.provider_snapshot->'loyalty','recharge_code',NEW.recharge_code,
          'previous_status',previous->>'status','previous_refund_amount',previous->'refund_amount');
    ELSIF TG_TABLE_NAME = 'redeem_codes' THEN
        IF data->>'status' <> 'used' OR (TG_OP = 'UPDATE' AND previous->>'status' = 'used') THEN RETURN NEW; END IF;
        data := data || jsonb_build_object('order',(
            SELECT jsonb_build_object('id',p.id,'amount',p.amount,'pay_amount',p.pay_amount,
              'currency',COALESCE(p.provider_snapshot->>'currency','CNY'),'loyalty',p.provider_snapshot->'loyalty')
            FROM payment_orders p WHERE p.recharge_code = data->>'code' LIMIT 1));
    ELSIF TG_TABLE_NAME = 'user_subscriptions' THEN
        IF TG_OP = 'UPDATE' AND NEW.term_version = OLD.term_version AND NEW.starts_at = OLD.starts_at
           AND NEW.expires_at = OLD.expires_at AND NEW.status = OLD.status AND NEW.deleted_at IS NOT DISTINCT FROM OLD.deleted_at THEN RETURN NEW; END IF;
        data := data || jsonb_build_object('previous',previous,'group_name',(SELECT name FROM groups WHERE id = NEW.group_id),
          'order',(SELECT jsonb_build_object('id',p.id,'amount',p.amount,'pay_amount',p.pay_amount,
            'payment_type',p.payment_type,'currency',COALESCE(p.provider_snapshot->>'currency','CNY'),'plan_id',p.plan_id,
            'plan_name',(SELECT name FROM subscription_plans WHERE id=p.plan_id))
            FROM regexp_split_to_table(NEW.notes,E'\\r?\\n') WITH ORDINALITY note(line,position)
            JOIN payment_orders p ON btrim(note.line) = 'payment order ' || p.id
            ORDER BY note.position DESC LIMIT 1));
    END IF;
    PERFORM business_event_emit(TG_TABLE_NAME || ':' || (data->>'id') || ':' || nextval('business_events_id_seq'),
        TG_TABLE_NAME,uid,clock_timestamp(),data || jsonb_build_object('operation',TG_OP));
    RETURN NEW;
END $$;
CREATE OR REPLACE TRIGGER business_payment AFTER INSERT OR UPDATE ON payment_orders FOR EACH ROW EXECUTE FUNCTION business_capture_source();
CREATE OR REPLACE TRIGGER business_redeem AFTER INSERT OR UPDATE ON redeem_codes FOR EACH ROW EXECUTE FUNCTION business_capture_source();
CREATE OR REPLACE TRIGGER business_subscription AFTER INSERT OR UPDATE ON user_subscriptions FOR EACH ROW EXECUTE FUNCTION business_capture_source();
CREATE OR REPLACE TRIGGER business_checkin AFTER INSERT OR UPDATE ON daily_checkin_records FOR EACH ROW EXECUTE FUNCTION business_capture_source();
CREATE OR REPLACE TRIGGER business_welfare AFTER INSERT OR UPDATE ON welfare_records FOR EACH ROW EXECUTE FUNCTION business_capture_source();
CREATE OR REPLACE TRIGGER business_lottery_balance AFTER INSERT ON balance_transactions FOR EACH ROW EXECUTE FUNCTION business_capture_source();
CREATE OR REPLACE TRIGGER business_lottery_draw AFTER INSERT ON lottery_draws FOR EACH ROW EXECUTE FUNCTION business_capture_source();
CREATE OR REPLACE TRIGGER business_ticket AFTER INSERT OR UPDATE ON lottery_ticket_ledger FOR EACH ROW EXECUTE FUNCTION business_capture_source();
CREATE OR REPLACE TRIGGER business_affiliate AFTER INSERT ON user_affiliate_ledger FOR EACH ROW EXECUTE FUNCTION business_capture_source();

CREATE OR REPLACE FUNCTION business_emit_usage(data JSONB) RETURNS VOID LANGUAGE plpgsql AS $$
DECLARE pool JSONB; rule JSONB; event_key TEXT; at_time TIMESTAMPTZ;
BEGIN
    at_time := COALESCE((data->>'created_at')::TIMESTAMPTZ,clock_timestamp());
    event_key := 'usage:' || (data->>'api_key_id') || ':' || COALESCE(NULLIF(data->>'request_id',''),'log:' || (data->>'id'));
    SELECT to_jsonb(p) INTO pool FROM business_cost_bindings b JOIN business_cost_pools p ON p.id=b.pool_id
      WHERE b.account_id=(data->>'account_id')::BIGINT AND b.effective_at<=at_time ORDER BY b.effective_at DESC,b.id DESC LIMIT 1;
    SELECT to_jsonb(r) INTO rule FROM business_cost_rules r WHERE r.pool_id=(pool->>'id')::BIGINT
      AND r.effective_at<=at_time AND r.model IN ('*',COALESCE(NULLIF(data->>'upstream_model',''),data->>'model'))
      AND r.service_tier IN ('',COALESCE(data->>'service_tier','')) AND r.image_size IN ('',COALESCE(data->>'image_size',''))
      AND r.video_resolution IN ('',COALESCE(data->>'video_resolution',''))
      ORDER BY (r.model<>'*') DESC,((r.service_tier<>'')::int+(r.image_size<>'')::int+(r.video_resolution<>'')::int) DESC,r.effective_at DESC,r.id DESC LIMIT 1;
    data := data || jsonb_build_object('pool',pool,'rule',rule,
      'account_name',(SELECT name FROM accounts WHERE id=(data->>'account_id')::BIGINT),
      'account_type',(SELECT type FROM accounts WHERE id=(data->>'account_id')::BIGINT),
      'group_name',(SELECT name FROM groups WHERE id=(data->>'group_id')::BIGINT),
      'user_role',(SELECT role FROM users WHERE id=(data->>'user_id')::BIGINT));
    PERFORM business_event_emit(event_key,'usage',(data->>'user_id')::BIGINT,at_time,data);
END $$;
CREATE OR REPLACE FUNCTION business_capture_usage() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    PERFORM business_emit_usage(to_jsonb(NEW));
    RETURN NEW;
END $$;
CREATE OR REPLACE TRIGGER business_usage AFTER INSERT ON usage_logs FOR EACH ROW EXECUTE FUNCTION business_capture_usage();

-- Honest opening positions, already in the post-239 site-credit unit.
INSERT INTO business_events (source_key,event_type,user_id,occurred_at,payload)
SELECT 'opening-wallet:' || id,'opening_unknown',id,NOW(),
 jsonb_build_object('credits',balance+COALESCE(frozen_balance,0),'frozen',COALESCE(frozen_balance,0),'user_name',username)
FROM users WHERE balance<>0 OR COALESCE(frozen_balance,0)<>0 ON CONFLICT(source_key) DO NOTHING;
INSERT INTO business_events (source_key,event_type,user_id,occurred_at,payload)
SELECT 'opening-subscription:' || id,'user_subscriptions',user_id,NOW(),
 to_jsonb(user_subscriptions) || jsonb_build_object('opening',true,'original_starts_at',starts_at,'starts_at',NOW())
FROM user_subscriptions WHERE deleted_at IS NULL AND expires_at>NOW() ON CONFLICT(source_key) DO NOTHING;

-- Ledger history and price versions are append-only. Corrections are events.
CREATE OR REPLACE FUNCTION business_reject_history_mutation() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'business history is append-only; record a correction or reversal'; END $$;
CREATE OR REPLACE TRIGGER business_events_immutable BEFORE UPDATE OR DELETE ON business_events
FOR EACH ROW EXECUTE FUNCTION business_reject_history_mutation();
CREATE OR REPLACE TRIGGER business_rules_immutable BEFORE UPDATE OR DELETE ON business_cost_rules
FOR EACH ROW EXECUTE FUNCTION business_reject_history_mutation();
CREATE OR REPLACE TRIGGER business_bindings_immutable BEFORE UPDATE OR DELETE ON business_cost_bindings
FOR EACH ROW EXECUTE FUNCTION business_reject_history_mutation();

-- A batch reservation keeps its own funding composition. The dedup row and
-- wallet movement commit together, including release/capture retries.
CREATE OR REPLACE FUNCTION business_capture_batch_billing() RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE batch_key TEXT; action_name TEXT; job JSONB; uid BIGINT; gid BIGINT; data JSONB;
BEGIN
 IF NEW.request_id !~ '^batch_image_(hold|capture|release):' THEN RETURN NEW; END IF;
 batch_key := split_part(NEW.request_id,':',2);
 action_name := split_part(split_part(NEW.request_id,':',1),'_',3);
 SELECT jsonb_build_object('account_id',account_id,'model',model,'image_count',success_count,
   'actual_cost',actual_cost,'total_cost',actual_cost,'account_rate_multiplier',account_rate_multiplier)
 INTO job FROM batch_image_jobs WHERE batch_id=batch_key;
 SELECT user_id,group_id INTO uid,gid FROM api_keys WHERE id=NEW.api_key_id;
 data := COALESCE(job,'{}'::jsonb) || jsonb_build_object('batch_id',batch_key,'action',action_name,
   'user_id',uid,'group_id',gid,'api_key_id',NEW.api_key_id,'request_id',NEW.request_id,
   'created_at',clock_timestamp(),'billing_applied',true,'billing_type',0);
 PERFORM business_event_emit('batch:' || NEW.api_key_id || ':' || NEW.request_id,'batch_operation',uid,clock_timestamp(),data);
 IF action_name='capture' THEN PERFORM business_emit_usage(data); END IF;
 RETURN NEW;
END $$;
CREATE OR REPLACE TRIGGER business_batch_billing AFTER INSERT ON usage_billing_dedup
FOR EACH ROW EXECUTE FUNCTION business_capture_batch_billing();
