-- Freeze the legacy ledger without deleting its records or views.
CREATE TABLE IF NOT EXISTS operating_legacy_archive (
 id SMALLINT PRIMARY KEY CHECK(id=1), cutover_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
INSERT INTO operating_legacy_archive(id) VALUES(1) ON CONFLICT DO NOTHING;
DO $$ DECLARE r RECORD; BEGIN
 FOR r IN SELECT t.tgname, n.nspname, c.relname FROM pg_trigger t
 JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace
 WHERE n.nspname='public' AND NOT t.tgisinternal AND t.tgname LIKE 'business_%'
 AND c.relname NOT LIKE 'business_%'
 LOOP EXECUTE format('DROP TRIGGER %I ON %I.%I',r.tgname,r.nspname,r.relname); END LOOP;
END $$;

CREATE TABLE IF NOT EXISTS operating_settings (
 id SMALLINT PRIMARY KEY CHECK(id=1), enabled_at TIMESTAMPTZ,
 timezone TEXT NOT NULL DEFAULT 'Asia/Shanghai', quota_timezone TEXT,
 opening_revenue NUMERIC(24,8) NOT NULL DEFAULT 0 CHECK(opening_revenue>=0),
 opening_private_revenue NUMERIC(24,8) NOT NULL DEFAULT 0 CHECK(opening_private_revenue>=0),
 opening_cost NUMERIC(24,8) NOT NULL DEFAULT 0 CHECK(opening_cost>=0),
 actor_id BIGINT, created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
INSERT INTO operating_settings(id) VALUES(1) ON CONFLICT DO NOTHING;
CREATE TABLE IF NOT EXISTS operating_records (
 id BIGSERIAL PRIMARY KEY, source_key TEXT NOT NULL UNIQUE,
 kind TEXT NOT NULL CHECK(kind IN ('cost','income','receipt','income_adjustment','cost_adjustment')),
 amount NUMERIC(24,8) NOT NULL CHECK(amount<>0), occurred_on DATE NOT NULL,
 category TEXT NOT NULL DEFAULT '', notes TEXT NOT NULL DEFAULT '', actor_id BIGINT NOT NULL,
 reverses_id BIGINT UNIQUE REFERENCES operating_records(id), created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX IF NOT EXISTS operating_records_date ON operating_records(occurred_on);
CREATE TABLE IF NOT EXISTS operating_terms (
 id BIGSERIAL PRIMARY KEY, subscription_id BIGINT NOT NULL, term_version BIGINT NOT NULL,
 user_id BIGINT NOT NULL, group_id BIGINT NOT NULL, group_name TEXT NOT NULL, platform TEXT NOT NULL,
 order_id BIGINT, plan_id BIGINT, plan_name TEXT NOT NULL DEFAULT '',
 starts_at TIMESTAMPTZ NOT NULL, ends_at TIMESTAMPTZ NOT NULL,
 amount NUMERIC(24,8) CHECK(amount>=0), quota NUMERIC(24,8) CHECK(quota>0),
 baseline_units NUMERIC(24,8) NOT NULL DEFAULT 0, opening BOOLEAN NOT NULL DEFAULT false,
 funding TEXT NOT NULL DEFAULT 'unknown' CHECK(funding IN ('paid','gift','unknown')),
 current_term BOOLEAN NOT NULL DEFAULT true, notes TEXT NOT NULL DEFAULT '',
 configured_by BIGINT, created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX IF NOT EXISTS operating_terms_subscription ON operating_terms(subscription_id,term_version,id DESC);
CREATE UNIQUE INDEX IF NOT EXISTS operating_terms_current ON operating_terms(subscription_id) WHERE current_term;
CREATE TABLE IF NOT EXISTS operating_usage (
 source_key TEXT PRIMARY KEY, occurred_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 user_id BIGINT NOT NULL, group_id BIGINT NOT NULL DEFAULT 0, term_id BIGINT REFERENCES operating_terms(id),
 balance_amount NUMERIC(24,8) NOT NULL DEFAULT 0, subscription_units NUMERIC(24,8) NOT NULL DEFAULT 0,
 tokens BIGINT NOT NULL DEFAULT 0, CHECK(balance_amount>=0 AND subscription_units>=0)
);
CREATE INDEX IF NOT EXISTS operating_usage_time ON operating_usage(occurred_at);
CREATE TABLE IF NOT EXISTS operating_daily_usage (
 day DATE NOT NULL, group_id BIGINT NOT NULL, term_id BIGINT NOT NULL DEFAULT 0,
 balance_amount NUMERIC(24,8) NOT NULL DEFAULT 0, subscription_units NUMERIC(24,8) NOT NULL DEFAULT 0,
 tokens BIGINT NOT NULL DEFAULT 0, requests BIGINT NOT NULL DEFAULT 0,
 PRIMARY KEY(day,group_id,term_id)
);
CREATE TABLE IF NOT EXISTS operating_rewards (
 source_key TEXT PRIMARY KEY, amount NUMERIC(24,8) NOT NULL CHECK(amount>0),
 source TEXT NOT NULL, occurred_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX IF NOT EXISTS operating_rewards_time ON operating_rewards(occurred_at);

CREATE OR REPLACE FUNCTION operating_snapshot_term(sid BIGINT, is_opening BOOLEAN DEFAULT false)
RETURNS VOID LANGUAGE plpgsql AS $$
DECLARE s RECORD; g RECORD; o RECORD; existing RECORD; q NUMERIC; p NUMERIC;
 cfg RECORD; finance_start TIMESTAMPTZ; funding_kind TEXT := 'unknown';
BEGIN
 SELECT * INTO cfg FROM operating_settings WHERE id=1;
 IF cfg.enabled_at IS NULL THEN RETURN; END IF;
 SELECT * INTO s FROM user_subscriptions WHERE id=sid;
 SELECT * INTO g FROM groups WHERE id=s.group_id;
 SELECT po.*, sp.name AS plan_name, sp.currency AS plan_currency INTO o
 FROM regexp_split_to_table(COALESCE(s.notes,''),E'\\r?\\n') WITH ORDINALITY n(line,pos)
 JOIN payment_orders po ON btrim(n.line)='payment order '||po.id
 LEFT JOIN subscription_plans sp ON sp.id=po.plan_id ORDER BY n.pos DESC LIMIT 1;
 SELECT * INTO existing FROM operating_terms WHERE subscription_id=sid AND current_term FOR UPDATE;
 IF existing.id IS NOT NULL AND existing.term_version=s.term_version AND existing.order_id IS NOT DISTINCT FROM o.id THEN
   UPDATE operating_terms SET ends_at=s.expires_at WHERE id=existing.id;
   RETURN;
 END IF;
 finance_start := CASE WHEN is_opening THEN cfg.enabled_at ELSE clock_timestamp() END;
 UPDATE operating_terms SET current_term=false, ends_at=LEAST(ends_at,finance_start) WHERE subscription_id=sid AND current_term;
 q := NULLIF(g.subscription_total_limit_usd,0);
 IF g.subscription_quota_reset_mode='until_subscription_expires' THEN
   q := LEAST(q,NULLIF(g.daily_limit_usd,0),NULLIF(g.weekly_limit_usd,0),NULLIF(g.monthly_limit_usd,0));
 END IF;
 IF q IS NULL AND g.daily_limit_usd>0 AND g.weekly_limit_usd IS NULL AND g.monthly_limit_usd IS NULL THEN
   IF s.expires_at<=s.starts_at+interval '24 hours' OR g.subscription_quota_reset_mode='until_subscription_expires' THEN
     q := g.daily_limit_usd;
   ELSIF cfg.quota_timezone IS NOT NULL THEN
     q := g.daily_limit_usd * (((s.expires_at-interval '1 microsecond') AT TIME ZONE cfg.quota_timezone)::date-(s.starts_at AT TIME ZONE cfg.quota_timezone)::date+1);
   END IF;
 END IF;
 IF o.id IS NOT NULL THEN
   funding_kind := 'paid';
   IF o.payment_type='balance' OR COALESCE(NULLIF(o.provider_snapshot->>'currency',''),NULLIF(o.plan_currency,''),'CNY')='CNY' THEN p := o.pay_amount; END IF;
 END IF;
 INSERT INTO operating_terms(subscription_id,term_version,user_id,group_id,group_name,platform,order_id,plan_id,plan_name,
 starts_at,ends_at,amount,quota,baseline_units,opening,funding)
 VALUES(s.id,s.term_version,s.user_id,s.group_id,g.name,g.platform,o.id,o.plan_id,COALESCE(o.plan_name,g.name),
 finance_start,s.expires_at,p,q,CASE WHEN is_opening THEN s.total_usage_usd ELSE 0 END,is_opening,funding_kind);
END $$;
CREATE OR REPLACE FUNCTION operating_subscription_changed() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN PERFORM operating_snapshot_term(NEW.id,false); RETURN NEW; END $$;
CREATE OR REPLACE TRIGGER operating_subscription AFTER INSERT OR UPDATE OF term_version,starts_at,expires_at,notes
 ON user_subscriptions FOR EACH ROW EXECUTE FUNCTION operating_subscription_changed();

-- This records settlement facts, never per-request profit or upstream cost.
CREATE OR REPLACE FUNCTION operating_record_usage(k TEXT, uid BIGINT, gid BIGINT, sid BIGINT, ver BIGINT, balance NUMERIC, units NUMERIC, token_count BIGINT)
RETURNS VOID LANGUAGE plpgsql AS $$
DECLARE cfg RECORD; at_time TIMESTAMPTZ := clock_timestamp(); tid BIGINT; inserted_key TEXT;
BEGIN
 SELECT * INTO cfg FROM operating_settings WHERE id=1;
 IF cfg.enabled_at IS NULL OR (balance<=0 AND units<=0) THEN RETURN; END IF;
 IF units>0 THEN
   SELECT id INTO tid FROM operating_terms WHERE subscription_id=sid AND term_version=ver ORDER BY id DESC LIMIT 1;
   IF tid IS NULL THEN
     PERFORM operating_snapshot_term(sid,false);
     SELECT id INTO tid FROM operating_terms WHERE subscription_id=sid AND term_version=ver ORDER BY id DESC LIMIT 1;
   END IF;
 END IF;
 INSERT INTO operating_usage(source_key,occurred_at,user_id,group_id,term_id,balance_amount,subscription_units,tokens)
 VALUES(k,at_time,uid,COALESCE(gid,0),tid,balance,units,token_count) ON CONFLICT DO NOTHING RETURNING source_key INTO inserted_key;
 IF inserted_key IS NULL THEN RETURN; END IF;
 INSERT INTO operating_daily_usage(day,group_id,term_id,balance_amount,subscription_units,tokens,requests)
 VALUES((at_time AT TIME ZONE cfg.timezone)::date,COALESCE(gid,0),COALESCE(tid,0),balance,units,token_count,1)
 ON CONFLICT(day,group_id,term_id) DO UPDATE SET
 balance_amount=operating_daily_usage.balance_amount+EXCLUDED.balance_amount,
 subscription_units=operating_daily_usage.subscription_units+EXCLUDED.subscription_units,
 tokens=operating_daily_usage.tokens+EXCLUDED.tokens,requests=operating_daily_usage.requests+1;
END $$;
CREATE OR REPLACE FUNCTION operating_record_reward(k TEXT, value NUMERIC, reward_source TEXT)
RETURNS VOID LANGUAGE plpgsql AS $$
BEGIN
 IF value>0 AND EXISTS(SELECT 1 FROM operating_settings WHERE id=1 AND enabled_at IS NOT NULL) THEN
 INSERT INTO operating_rewards(source_key,amount,source) VALUES(k,value,reward_source) ON CONFLICT DO NOTHING;
 END IF;
END $$;
CREATE OR REPLACE FUNCTION operating_reward_changed() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
 IF TG_TABLE_NAME='users' THEN
   PERFORM operating_record_reward('signup:'||NEW.id,NEW.balance,'新人注册');
 ELSIF TG_TABLE_NAME='redeem_codes' THEN
   IF NEW.status='used' AND (TG_OP='INSERT' OR OLD.status IS DISTINCT FROM 'used') AND NEW.value>0
      AND (NEW.type IN ('daily_checkin','usage_rebate','affiliate','registration','qq_binding_reward','admin_gift')
       OR (NEW.type='balance' AND EXISTS(SELECT 1 FROM lottery_draws WHERE reward_ref=NEW.code))) THEN
     PERFORM operating_record_reward('redeem:'||NEW.id,NEW.value,NEW.type);
   ELSIF NEW.status='used' AND (TG_OP='INSERT' OR OLD.status IS DISTINCT FROM 'used') AND NEW.type='balance' THEN
     -- A paid top-up may include a bonus; only the excess is a gift.
     PERFORM operating_record_reward('redeem:'||NEW.id,GREATEST(NEW.value-po.pay_amount,0),'recharge_bonus')
     FROM payment_orders po WHERE po.recharge_code=NEW.code AND po.order_type='balance'
       AND COALESCE(NULLIF(po.provider_snapshot->>'currency',''),'CNY')='CNY';
   END IF;
 ELSIF TG_TABLE_NAME='promo_code_usages' THEN
   PERFORM operating_record_reward('promo:'||NEW.id,NEW.bonus_amount,'promo');
 ELSIF TG_TABLE_NAME='user_affiliate_ledger' THEN
   IF NEW.action='transfer' THEN PERFORM operating_record_reward('affiliate:'||NEW.id,NEW.amount,'affiliate'); END IF;
 ELSIF TG_TABLE_NAME='balance_transactions' THEN
   IF NEW.transaction_type IN ('lottery_reward','qq_bind_welcome_bonus') THEN
     PERFORM operating_record_reward('balance:'||NEW.id,NEW.amount,NEW.transaction_type);
   END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE OR REPLACE TRIGGER operating_signup AFTER INSERT ON users FOR EACH ROW EXECUTE FUNCTION operating_reward_changed();
CREATE OR REPLACE TRIGGER operating_redeem AFTER INSERT OR UPDATE ON redeem_codes FOR EACH ROW EXECUTE FUNCTION operating_reward_changed();
CREATE OR REPLACE TRIGGER operating_balance_reward AFTER INSERT ON balance_transactions FOR EACH ROW EXECUTE FUNCTION operating_reward_changed();

CREATE OR REPLACE TRIGGER operating_promo AFTER INSERT ON promo_code_usages FOR EACH ROW EXECUTE FUNCTION operating_reward_changed();
CREATE OR REPLACE TRIGGER operating_affiliate AFTER INSERT ON user_affiliate_ledger FOR EACH ROW EXECUTE FUNCTION operating_reward_changed();
CREATE INDEX IF NOT EXISTS operating_daily_usage_term ON operating_daily_usage(term_id,day);
